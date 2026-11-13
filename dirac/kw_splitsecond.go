package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Split second (CR 702.61a): while a spell with it is on the stack, players
// can't cast spells or activate abilities that aren't mana abilities. gorge
// enforces it by withholding those options from the priority offer
// (rules split_second.go filterSplitSecondActions), keeping the land play, the
// mana abilities and the face-up turn (Station and Crew are activated abilities,
// CR 702.184a, 702.122a, and are withheld).
// The keyword exists in the kernel as a StatStackKw grant (Samut, Tyrant of
// Naktamun's instants and sorceries), read live off each spell on the stack.
// Cold: behind genPrep.anySplitSecond.

// DebugNoSplitSecond (a mutation switch): the priority offer is left whole.
var DebugNoSplitSecond bool

// splitSecondHolds reports whether a spell on the stack has split second.
//
//go:noinline
func (s *State) splitSecondHolds(k *prep) bool {
	if DebugNoSplitSecond {
		return false
	}
	for i := 0; i < int(s.sp); i++ {
		e := &s.stack[i]
		if e.kind != seSpell {
			continue
		}
		if n, _ := s.stackKwN(k, kop.StackKwSplitSecond, e.card, e.obj, e.ctrl, true); n > 0 {
			return true
		}
	}
	return false
}

// filterSplitSecond drops every cast and every non-mana ability option from d
// (order kept).
//
//go:noinline
func (s *State) filterSplitSecond(k *prep, d *Decision) {
	n := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if s.splitSecondBars(k, a) {
			continue
		}
		d.Options[n] = a
		n++
	}
	d.N = uint16(n)
}

func (s *State) splitSecondBars(k *prep, a Action) bool {
	switch a.Kind() {
	case AkPlot:
		return a.Mode() != plotModeSuspend // suspend exiles a card, it casts no spell (CR 702.62a)
	case AkCast, AkCastCasualty, AkCastConspire, AkSneak, AkWebSling, AkBestow, AkActivate:
		return true
	}
	return false
}
