package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The exiled-with source link (CR 406.6 and 607.2a: a linked
// ability refers to the cards exiled "with" its source; CR 400.7: a source that
// changes zone is a new object and the link does not follow it). gorge keeps
// two associations and one reader, exiledBySourceAt (engine/effects/context.go):
//
//   - the forward list src.ExiledCards, written by a ChangeZone object loop, a
//     library search and ChangeZoneAll exile of a non-token card, with no zone
//     or incarnation check, cleared when the source enters the battlefield
//     other than from the stack;
//   - the reverse scalar Object.ExiledWith (+ExiledWithInc), set only for a
//     source whose card text names ExiledWithSource (exileProvenanceNeeded) or
//     for a Hideaway's face-down exile, cleared by every other move, and live
//     while the source is the incarnation that exiled the card or a resolving
//     trigger fired from it.
//
// State's cold exw holds one record per (card, source): fl bit 0 the reverse link,
// bit 1 the forward one. Only a card that reads the link (CardDef.ExiledWith,
// genPrep.exwRec) records, and only inside a resolution, except the cast
// spell's own BeholdExile cost, whose exile carries the spell as its source in
// gorge (EmitChoiceCosts; kernel act_beholdexile.go writes that record
// through exwPut). gorge's Incarnation advances on a battlefield
// crossing; the kernel's object ver on every zone change, so the stamp is the
// source's ver at the exile; the rule is that of gen.links
// (zone_rememberpersist.go): live at the same ver, or, for a resolving trigger,
// the ver it fired from (+1: a leaves trigger queued after the departure).
// Lockstep proves the fire arm (DebugExwNoFire diverges) and the ver test
// (DebugExwIgnoreVer diverges). The forward list outlives a departed source on
// its own, so a source that has left reads through it; a reverse-only link read
// by a non-trigger ability after the source left is not modelled and no lifted
// card has the shape.

const (
	exwRev uint8 = 1 << iota // the reverse scalar is live
	exwFwd                   // the forward ExiledCards entry is live
	exwTok                   // o is a token (no forward entry; gorge records none)
)

// genPrep.exwRec bits: the card reads the link; its text names
// ExiledWithSource (CardDef.ExiledWithRev, gorge exileProvenanceNeeded).
const (
	exwRecRead uint8 = 1 << iota
	exwRecRev
)

// maxExw bounds the links of a table with a card that records them.
const maxExw = 64

// coldExw is the links, for a table whose cards record them (exwPut's
// callers); an empty one is simply full.
var coldExw = newCold[exwRec]("exw", func(g *genPrep) int {
	if g.anyExW || g.anyFaceExile {
		return maxExw
	}
	return 0
}).view(func(c *coldArrays) *[]exwRec { return &c.exw })

// exwRec is one exiled-with link: card o exiled by src (stamp sver).
type exwRec struct {
	o, src Obj
	sver   uint8
	fl     uint8
}

// DebugExwIgnoreVer is the mutation hook (TestExwMutation): the reverse link
// is live whatever the source's ver, a flickered source still finds its cards.
var DebugExwIgnoreVer bool

// DebugExwNoFire drops the resolving trigger's fire-ver arm of the reader
// (TestExwMutation): a leaves trigger no longer finds its cards.
var DebugExwNoFire bool

// DebugExwAnySource makes every card in exile read as exiled with the reading
// source (TestBorgExileMutation): Assimilation Assessor's choice then offers
// the cards another source exiled.
var DebugExwAnySource bool

// ExwStats counts the link's lockstep coverage (TestExwLockstep).
type ExwStats struct {
	Recorded   int // links written (a card exiled by a reading source)
	Reads      int // PredExiledWith reads
	Matched    int // reads that matched
	Rejected   int // reads that rejected a card another source exiled
	AfterLeft  int // matched reads after the source left the battlefield
	FwdCleared int // forward bits cleared by a re-entry
	CountPos   int // ExiledWith$Amount reads that counted at least one card
	RevCleared int // reverse bits cleared by a card leaving exile or being re-exiled

	// RecBy / MatchBy: links written / reads that matched, by the reading
	// card (the source's card index; cards past the array are not counted).
	RecBy, MatchBy [64]int
}

// DebugExwStats, when non-nil, accumulates the counters above.
var DebugExwStats *ExwStats

// exwProvOp: the movers that set the reverse scalar whatever the card's text
// says: a Hideaway's face-down exile carries its source in the move marker
// (gorge apply_zone "exiled_with_face_down_maylook").
func exwProvOp(in *kop.Instr) bool {
	return in.Op == kop.OpExt && in.Ext() == kop.ExtHideaway && in.C == 0
}

// exwForwardOp: the movers whose exile gorge also writes to the source's
// ExiledCards list (the ChangeZone object loop, ChangeZoneAll and a library
// search); a Dig or any other effect exile sets the reverse scalar only.
func exwForwardOp(op kop.Op) bool {
	switch op {
	case kop.OpExile, kop.OpExileEx, kop.OpChangeZoneAll, kop.OpChangeZoneAllEx, kop.OpSearchEx, kop.OpChooseMove, kop.OpExileUntilLeaves:
		return true
	}
	return false
}

// exwOpAt is the instruction the resolution is executing (nil when none).
func (s *State) exwOpAt(k *prep) *kop.Instr {
	pr := s.prog(k)
	if int(s.res.pc) >= len(pr) {
		return nil
	}
	return &pr[s.res.pc]
}

// exwExile runs when o is placed into exile (placeFromK, genPrep.anyExW
// only): the card is a new object, so every reverse link to it ends (gorge
// overwrites the scalar with the new exiler, or zero); a resolution of a
// reading card then records its own.
//
//go:noinline
func (s *State) exwExile(k *prep, o Obj) {
	g := &s.g
	for i := 0; i < int(g.nexw); i++ {
		if r := &s.cold.exw[i]; r.o == o && r.fl&exwRev != 0 {
			r.fl &^= exwRev
			if DebugExwStats != nil {
				DebugExwStats.RevCleared++
			}
		}
	}
	if !s.res.active || k.gen.exwRec[s.res.e.card] == 0 {
		return
	}
	e := &s.res.e
	src := e.obj
	if src == NoObj {
		return
	}
	// gorge writes the forward list for a ChangeZone object loop, a library
	// search and ChangeZoneAll exile of a non-token card, and the reverse
	// scalar only when the card's text names ExiledWithSource (or a
	// Hideaway's marker): a card with neither records nothing
	var fl uint8
	in := s.exwOpAt(k)
	if k.gen.exwRec[e.card]&exwRecRev != 0 || in != nil && exwProvOp(in) {
		fl = exwRev
	}
	if tok := s.objs[o].of&ofToken != 0; tok {
		if fl != 0 {
			fl |= exwTok
		}
	} else if in != nil && exwForwardOp(in.Op) {
		fl |= exwFwd
	}
	if fl == 0 {
		return
	}
	s.exwPut(o, src, s.objs[src].ver, fl, int(e.card))
}

// exwPut writes (or refreshes) the record of card o exiled by src, stamped
// sver, for the reading card `card` (the stats' key). A full pool is
// StallExiledWith.
func (s *State) exwPut(o, src Obj, sver, fl uint8, card int) {
	g := &s.g
	for i := 0; i < int(g.nexw); i++ {
		if r := &s.cold.exw[i]; r.o == o && r.src == src {
			r.fl, r.sver = r.fl&exwFwd|fl, sver
			if DebugExwStats != nil {
				DebugExwStats.noteRec(card)
			}
			return
		}
	}
	if int(g.nexw) >= len(s.cold.exw) {
		n := 0
		for i := 0; i < int(g.nexw); i++ {
			if s.cold.exw[i].fl&(exwRev|exwFwd|exwFace) != 0 {
				s.cold.exw[n] = s.cold.exw[i]
				n++
			}
		}
		g.nexw = uint8(n)
		if n >= len(s.cold.exw) {
			s.capFull(StallExiledWith)
			return
		}
	}
	s.cold.exw[g.nexw] = exwRec{o: o, src: src, sver: sver, fl: fl}
	g.nexw++
	if DebugExwStats != nil {
		DebugExwStats.noteRec(card)
	}
}

func (st *ExwStats) noteRec(card int) {
	st.Recorded++
	if card >= 0 && card < len(st.RecBy) {
		st.RecBy[card]++
	}
}

// exwLeaveExile runs when o leaves the exile zone (detach): gorge clears
// Object.ExiledWith on every other move, and prunes o from every source's
// forward ExiledCards list (apply_zone.go: a card that leaves exile is a new
// object for the association, "even if a later effect exiles the same engine
// ObjID"). A card cast from exile through a static's permission
// that later returns to exile through another source (two Ronas) is therefore
// no longer linked to the first one.
//
//go:noinline
func (s *State) exwLeaveExile(o Obj) {
	g := &s.g
	for i := 0; i < int(g.nexw); i++ {
		if r := &s.cold.exw[i]; r.o == o && r.fl&(exwRev|exwFwd|exwFace) != 0 {
			if r.fl&exwRev != 0 && DebugExwStats != nil {
				DebugExwStats.RevCleared++
			}
			r.fl &^= exwRev | exwFwd | exwFace | exwLook1
		}
	}
}

// exwEnter runs when object o enters the battlefield (enterGen): its forward
// list is cleared unless it came from the stack (gorge apply_zone.go: a spell
// becoming its permanent keeps its list).
//
//go:noinline
func (s *State) exwEnter(o Obj, from kop.Zone) {
	if from == kop.ZoneStack {
		return
	}
	g := &s.g
	for i := 0; i < int(g.nexw); i++ {
		r := &s.cold.exw[i]
		if r.src == o && r.fl&exwFwd != 0 {
			r.fl &^= exwFwd
			if DebugExwStats != nil {
				DebugExwStats.FwdCleared++
			}
		}
	}
}

// exwHas is gorge's exiledBySourceAt: was o exiled with src (the reading
// ability's source; trig/fire the resolving trigger's fire ver).
func (s *State) exwHas(o, src Obj, trig bool, fire uint8) bool {
	if o == NoObj || src == NoObj {
		return false
	}
	if DebugExwAnySource && s.objs[o].zone == kop.ZoneExile {
		return true
	}
	ver := s.objs[src].ver
	for i := 0; i < int(s.g.nexw); i++ {
		r := &s.cold.exw[i]
		if r.o != o || r.src != src {
			continue
		}
		if r.fl&exwFwd != 0 {
			return true
		}
		if r.fl&exwRev == 0 {
			continue
		}
		if DebugExwIgnoreVer || ver == r.sver ||
			!DebugExwNoFire && trig && (fire == r.sver || fire == r.sver+1) {
			return true
		}
	}
	return false
}

// exwCard is PredExiledWith for object o read by the resolving entry: the
// source is the entry's. Outside a resolution there is no source and nothing
// matches (kgen lowers the predicate into resolution-time movers only).
func (s *State) exwCard(o Obj) bool {
	if !s.res.active || o == NoObj {
		return false
	}
	e := &s.res.e
	ok := s.exwHas(o, e.obj, e.kind == seTrigger, e.srcVer)
	if st := DebugExwStats; st != nil {
		st.Reads++
		if ok {
			st.Matched++
			if c := int(s.res.e.card); c >= 0 && c < len(st.MatchBy) {
				st.MatchBy[c]++
			}
			if s.objs[e.obj].zone != kop.ZoneBattlefield {
				st.AfterLeft++
			}
		} else if s.objs[o].zone == kop.ZoneExile {
			st.Rejected++ // a card in exile that another source (or none) exiled
		}
	}
	return ok
}

// exwCount is ExExiledWithN: the cards in every seat's exile zone exiled with
// the entry's source (gorge exiledWithSet, seat 0 first).
func (s *State) exwCount(ctx evalCtx) int {
	if ctx.src == NoObj {
		return 0
	}
	trig, fire := false, uint8(0)
	if ctx.e != nil {
		trig, fire = ctx.e.kind == seTrigger, ctx.e.srcVer
	}
	n := 0
	for p := 0; p < int(s.nSeats()); p++ {
		if s.out(uint8(p)) {
			continue
		}
		pl := &s.pl[p]
		for j := 0; j < int(pl.exileN); j++ {
			if o := pl.exile[j]; ctx.uncounted != o+1 && s.exwHas(o, ctx.src, trig, fire) {
				n++
			}
		}
	}
	if st := DebugExwStats; st != nil && n > 0 {
		st.CountPos++
		if c := int(s.objs[ctx.src].card); c >= 0 && c < len(st.MatchBy) {
			st.MatchBy[c]++
		}
	}
	return n
}

func init() { stallNames[StallExiledWith] = "exiledwith" }
