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
// State's cold object-set store (zone_objsets.go) holds one record per (card,
// source), kind osExw with the flags in kind's high nibble: exwRev the reverse
// link, exwFwd the forward one. Only a card that reads the link
// (CardDef.ExiledWith, genPrep.exwRec) records, and only inside a resolution,
// except the cast spell's own BeholdExile cost, whose exile carries the spell
// as its source in gorge (EmitChoiceCosts; kernel act_beholdexile.go writes
// that record through exwPut). gorge's Incarnation advances on a battlefield
// crossing; the kernel's object ver on every zone change, so the stamp is the
// source's ver at the exile; the rule is that of gen.links
// (zone_rememberpersist.go): live at the same ver, or, for a resolving trigger,
// the ver it fired from (+1: a leaves trigger queued after the departure).
// Lockstep proves the fire arm (DebugExwNoFire diverges) and the ver test
// (DebugExwIgnoreVer diverges). The forward list outlives a departed source on
// its own, so a source that has left reads through it; a reverse-only link read
// by a non-trigger ability after the source left is not modelled and no lifted
// card has the shape.

// The exiled-with flags, in osRec.kind's high nibble (zone_objsets.go);
// exwRev and exwFwd are independent (a refresh restates the rest, keeping
// the forward bit). A token records the same flag bits as any other object:
// gorge keeps no forward entry for it, so exwPut sets none.
const (
	exwRev uint8 = 1 << 4 // the reverse scalar is live
	exwFwd uint8 = 1 << 5 // the forward ExiledCards entry is live
)

// genPrep.exwRec bits: the card reads the link; its text names
// ExiledWithSource (CardDef.ExiledWithRev, gorge exileProvenanceNeeded).
const (
	exwRecRead uint8 = 1 << iota
	exwRecRev
)

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
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		if rec := &r[i]; rec.o == o && rec.osKind() == osExw && rec.kind&exwRev != 0 {
			rec.kind &^= exwRev
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
	if tok := s.objs[o].of&ofToken != 0; !tok && in != nil && exwForwardOp(in.Op) {
		fl |= exwFwd
	}
	if fl == 0 {
		return
	}
	s.exwPut(o, src, s.objs[src].ver, fl, int(e.card))
}

// exwPut writes (or refreshes) the record of card o exiled by src, stamped
// sver, for the reading card `card` (the stats' key). A full store is
// StallObjSets (the pool's own StallExiledWith number stays reserved).
func (s *State) exwPut(o, src Obj, sver, fl uint8, card int) {
	r := s.cold.objSets
	n := s.osLen()
	for i := 0; i < n; i++ {
		if rec := &r[i]; rec.o == o && rec.src == src && rec.osKind() == osExw {
			rec.kind, rec.sver = osExw|rec.osFlags()&exwFwd|fl, sver
			if DebugExwStats != nil {
				DebugExwStats.noteRec(card)
			}
			return
		}
	}
	if n >= len(r) {
		s.osGC()
		if n = s.osLen(); n >= len(r) {
			s.capFull(StallObjSets)
			return
		}
	}
	r[n] = osRec{o: o, src: src, sver: sver, kind: osExw | fl}
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
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		if rec := &r[i]; rec.o == o && rec.osKind() == osExw && rec.kind&(exwRev|exwFwd|exwFace) != 0 {
			if rec.kind&exwRev != 0 && DebugExwStats != nil {
				DebugExwStats.RevCleared++
			}
			rec.kind &^= exwRev | exwFwd | exwFace | exwLook1
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
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src == o && rec.osKind() == osExw && rec.kind&exwFwd != 0 {
			rec.kind &^= exwFwd
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
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.o != o || rec.src != src || rec.osKind() != osExw {
			continue
		}
		if rec.kind&exwFwd != 0 {
			return true
		}
		if rec.kind&exwRev == 0 {
			continue
		}
		if DebugExwIgnoreVer || ver == rec.sver ||
			!DebugExwNoFire && trig && (fire == rec.sver || fire == rec.sver+1) {
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

// exiledWithOwnerSeat is RefExiledWithOwner: the first seat in seat order
// whose exile zone holds a card exiled with the reading source (gorge
// definedPlayerIDs over DefinedPlayer$
// Player.HasCardsInExile_Card.ExiledWithSource_GE1, the shared player
// filter's HasCardsIn property). The read is exwHas -- the ONE reader
// exwCount (ExExiledWithN) and exwCard (PredExiledWith) already share -- so
// a player ref and a count or filter cannot disagree about which cards the
// link holds. 255 when no seat's exile holds one (gorge manifests dread for
// nobody).
//
// gorge resolves the same grammar over every living seat and acts on EACH
// match, while the ref is one seat. Several linked cards in ONE seat's exile
// agree (gorge asks that seat once). A SECOND seat holding a linked card is
// the shape the ref cannot represent: it latches StallLowering (an
// unreachable shape, never a silent divergence) and the first seat is
// returned. The corpus carrier (Unidentified Hovership's leave trigger,
// TargetMax$ 1) exiles at most one card, so no seat but one ever matches.
func (s *State) exiledWithOwnerSeat(ctx evalCtx) uint8 {
	if ctx.src == NoObj {
		return 255
	}
	trig, fire := false, uint8(0)
	if ctx.e != nil {
		trig, fire = ctx.e.kind == seTrigger, ctx.e.srcVer
	}
	first := uint8(255)
	for p := 0; p < int(s.nSeats()); p++ {
		if s.out(uint8(p)) {
			continue
		}
		pl := &s.pl[p]
		for j := 0; j < int(pl.exileN); j++ {
			if !s.exwHas(pl.exile[j], ctx.src, trig, fire) {
				continue
			}
			if first == 255 {
				first = uint8(p)
			} else if first != uint8(p) {
				s.capFull(StallLowering) // a second seat holds a linked card: gorge acts on every seat, the ref names one
			}
			break
		}
	}
	return first
}

func init() { stallNames[StallExiledWith] = "exiledwith" }
