package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// remEachOwnNonLand is ExRemEachOwnNonLand: the cards in the resolution's
// remembered set that are nonland and owned by the repeated player (gorge
// Remembered$Valid Card.nonLand+RememberedPlayerOwn, Kroxa, Titan of Death's
// Hunger's RepeatEach body gate: effRepeatEach binds the repeated player as
// Remembered for the body, so the fold counts that opponent's discarded
// nonland cards and the LoseLife runs exactly when the read is 0). Zero
// outside an OpRepeatEachPlayer (repEnd == 0), as ctrlEachPl. The kernel's
// OpRepeatEachPlayer binds the repeated player in s.g.eachPl and does not
// append it to s.g.rem (gorge appends the subject player to cc.Remembered),
// so every entry is a card and needs no non-card skip.
//
//go:noinline
func (s *State) remEachOwnNonLand(k *prep) int {
	if s.g.repEnd == 0 {
		return 0
	}
	n := 0
	for _, o := range s.g.rem[:s.g.nrem] {
		ob := &s.objs[o]
		if ob.owner() == s.g.eachPl && (DebugRemEachOwnAnyCard || k.t.Cards[ob.card].Types&kop.TLand == 0) {
			n++
		}
	}
	return n
}

// DebugRemEachOwnAnyCard makes remEachOwnNonLand count land cards too, so a
// land discard stops costing the life. Test-only: the repeatcheck lockstep's
// mutation proof.
var DebugRemEachOwnAnyCard bool
