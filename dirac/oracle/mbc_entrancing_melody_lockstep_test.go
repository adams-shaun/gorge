package oracle

import "testing"

// CR 605.3b, 601.2b-c: p1 held Entrancing Melody ({X}{U}{U}, target creature
// with mana value X) beside a Nomadic Elf ("{1}{G}: Add one mana of any
// color", repeatable) and four lands, no blue source. Two Elf conversions pay
// {U}{U} at X=0 (a Vampire token has mana value 0), so gorge offered the cast
// (recorded tail-44 game, decision 364). The kernel's X-bound sweep counted
// the Elf as one any-colour unit and refused the colours; gorge's census
// prices a repeatable converter unbounded, so only the total mana bounds it.
func TestMBCEntrancingMelodyReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "entrancing_melody_options")
}
