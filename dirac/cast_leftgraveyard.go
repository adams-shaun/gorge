package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a spell cast from a graveyard (flashback, escape)
// leaves it for the stack, and gorge's per-turn zone-add list (g.Entered)
// records that move like any other, so Count$LeftGraveyardThisTurn counts the
// card the moment it is on the stack (Essence Anchor's gate opens while the
// flashback spell is still unresolved). pushBuild calls this for a cast whose
// origin is the graveyard; the entry is the stack add, From the graveyard.
//
//go:noinline
func (s *State) noteCastFromGrave(o Obj) {
	s.noteEnter(o, kop.ZoneStack, kop.ZoneGraveyard, false)
}

// unnoteCastFromGrave undoes noteCastFromGrave when the cast is reversed
// (CR 733.1). gorge's "reversed" stack->origin move truncates g.Entered to
// the length recorded at the push (events/apply_zone.go PreStackEnteredLen),
// dropping the cast's entry, the reverse move's own entry and anything added
// in between. The cast's entry is the last {o, stack, graveyard} record (the
// object cannot be pushed again before this reversal), so truncating the
// ring there is that rule. A full ring has already stalled and the push may
// not have been recorded: leave it.
//
//go:noinline
func (s *State) unnoteCastFromGrave(o Obj) {
	if !s.g.enteredOn || s.g.enteredFull {
		return
	}
	for i := int(s.g.enteredN) - 1; i >= 0; i-- {
		if e := &s.cold.entered[i]; e.o == o && e.to() == uint8(kop.ZoneStack) && e.from() == uint8(kop.ZoneGraveyard) {
			s.g.enteredN = uint8(i)
			return
		}
	}
}
