package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Palace Jailer's exile "until an opponent becomes the monarch" (CR 610.3),
// copied from gorge: the ChangeZone to exile (zone_change.go
// exileUntilOpponentMonarchMet, CR 610.3b) and the SubAbility$ Effect's
// one-shot BecomeMonarch promise (effect.go monarchJailRemembered, resolved
// inline by rules/trigger_delayed.go).
//
// The promise is a genState.links entry with no source: src is the exiled
// card itself (a self-link, which no "source left" walk matches: the card is
// in exile, and a departure of it comes only after the link is gone), sver the
// card's ver as it was exiled, ts the anchor's seat + 1. The anchor is the
// Jailer's controller (the exile ability's controller), the player whose
// opponents' crowning returns the card. No State growth: the entry takes a
// links slot, ts being read otherwise only for an Aegis link.
//
//   - ExtMonarchJail: the target leaves the battlefield for exile and the
//     link is recorded, unless the exile ability's entry carries sfOppCrowned
//     (CR 610.3b: an opponent became the monarch after the ability was put on
//     the stack, so the "until" event has already happened and nothing is
//     exiled). A token ceases in exile and is never linked (CR 111.7), as
//     gorge's monarchJailRemembered keeps only objects in exile.
//   - becomeMonarch: every stack ability not controlled by the new monarch is
//     stamped sfOppCrowned, then each link whose anchor is not the new monarch
//     returns its card to the battlefield under its owner's control (a new
//     object), inline and in link order, and ends. A link whose card has left
//     exile since (gorge's ForgetOnMoved$ Exile) ends without a return.

// sfOppCrowned is a stack ability's "an opponent became the monarch since it
// was put on the stack" bit (gorge Object.PushMonarchSeq against
// Player.BecameMonarchSeq). sfCountered is read nowhere (a countered entry
// leaves the stack), so its bit is free for an ability's entry.
const sfOppCrowned = sfCountered

// DebugMonarchJailAnyCrown (TestMonarchJailerMutation): the anchor's own
// crowning returns the card too, as if the promise named every player.
var DebugMonarchJailAnyCrown bool

// MonarchJailStats counts the mechanic's events; oracle tests only.
type MonarchJailStats struct {
	Exiles  int // a creature exiled with a monarch link
	Skips   int // the exile found sfOppCrowned and exiled nothing (CR 610.3b)
	Returns int // a link returned its card
	Ended   int // a link ended because its card had left exile
}

// DebugMonarchJailStats is the test hook; nil in normal play.
var DebugMonarchJailStats *MonarchJailStats

// useMonJail is genPrep.uses' bit for a table with some ExtMonarchJail.
var useMonJail = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtMonarchJail {
					return true
				}
			}
		}
	}
	return false
})

// monarchJailOp is ExtMonarchJail.
//
//kmanifest:ext ExtMonarchJail
func (s *State) monarchJailOp(k *prep, in *kop.Instr) (ok, done bool) {
	e := &s.res.e
	st := DebugMonarchJailStats
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return true, true
	}
	if e.flags&sfOppCrowned != 0 {
		if st != nil {
			st.Skips++
		}
		return true, true
	}
	if !s.moveObj(k, in.Obj, o, kop.ZoneExile) || s.objs[o].zone != kop.ZoneExile || s.objs[o].of&ofToken != 0 {
		return true, true
	}
	s.monarchJailPrune()
	if s.g.nlinks >= uint8(len(s.g.links)) {
		s.capFull(StallLinks)
		return true, true
	}
	l := &s.g.links[s.g.nlinks]
	l.src, l.sver, l.obj, l.ts = o, s.objs[o].ver, o, uint16(e.ctrl)+1
	s.g.nlinks++
	if st != nil {
		st.Exiles++
	}
	return true, true
}

// monarchJailLink reports whether link i is a monarch promise.
func (s *State) monarchJailLink(i int) bool {
	l := &s.g.links[i]
	return l.src == l.obj
}

// monarchJailDrop removes link i, shifting the link masks with it.
func (s *State) monarchJailDrop(i int) {
	if s.g.linkHand|s.g.linkHide|s.g.linkAeg != 0 {
		s.dropLinkHand(i)
	}
	copy(s.g.links[i:], s.g.links[i+1:s.g.nlinks])
	s.g.nlinks--
}

// monarchJailPrune ends the promises whose card has left exile (ForgetOnMoved$
// Exile): they would otherwise hold their slot for the rest of the game.
func (s *State) monarchJailPrune() {
	for i := 0; i < int(s.g.nlinks); i++ {
		if !s.monarchJailLink(i) {
			continue
		}
		if x := &s.objs[s.g.links[i].obj]; x.zone != kop.ZoneExile || x.ver != s.g.links[i].sver {
			s.monarchJailDrop(i)
			i--
			if st := DebugMonarchJailStats; st != nil {
				st.Ended++
			}
		}
	}
}

// monarchJailCrowned runs as p becomes the monarch (becomeMonarch).
//
//go:noinline
func (s *State) monarchJailCrowned(k *prep, p uint8) {
	for i := 0; i < int(s.sp); i++ {
		if e := &s.stack[i]; e.kind != seSpell && e.ctrl != p {
			e.flags |= sfOppCrowned
		}
	}
	for i := 0; i < int(s.g.nlinks); i++ {
		if !s.monarchJailLink(i) {
			continue
		}
		l := s.g.links[i]
		x := &s.objs[l.obj]
		if x.zone != kop.ZoneExile || x.ver != l.sver {
			s.monarchJailPrune()
			i = -1
			continue
		}
		if uint8(l.ts) == p+1 && !DebugMonarchJailAnyCrown {
			continue // the Jailer's own controller took the crown: the card stays
		}
		if s.out(uint8(l.ts) - 1) {
			// the anchor left the game (CR 800.4a): the promise it controlled
			// is gone, so its departure's handoff crowning returns nothing
			s.monarchJailDrop(i)
			i--
			continue
		}
		s.monarchJailDrop(i)
		i--
		kind := uint8(0)
		if k.gen.has(x.card, gfEnterRepl) {
			kind = s.cloneAskKind(k, x.card)
		}
		if kind == 0 {
			kind = s.enterChoiceKind(k, x.card)
		}
		if kind != 0 {
			s.capFull(StallLinks) // an as-enters choice or a clone election would ask: not modelled
			continue
		}
		if st := DebugMonarchJailStats; st != nil {
			st.Returns++
		}
		from := s.detach(l.obj)
		s.enterLinkRet(k, l.obj, x.owner(), from)
	}
}
