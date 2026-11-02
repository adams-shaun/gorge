package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// partyRoles finds CR 700.8's four party roles among the matchup's
// subtypes (subOutside when a role is absent: only an all-creature-types
// permanent has it then).
func partyRoles(t *kop.Table) (r [4]uint8) {
	for i, name := range [4]string{"Cleric", "Rogue", "Warrior", "Wizard"} {
		r[i] = subOutside
		for j, s := range t.Subtypes {
			if s == name {
				r[i] = uint8(j)
			}
		}
	}
	return r
}

// partySize is CR 700.8's party size (gorge partySize): the most of the
// four roles that different creatures player p controls can fill, each
// creature filling one role it has.
//
//go:noinline
func (s *State) partySize(k *prep, p uint8) int {
	if k.gen == nil {
		return 0
	}
	var reach uint16 = 1 // bit set: a reachable set of filled roles
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.isCreature(k, sl) {
			continue
		}
		var roles uint8
		for i, r := range k.gen.partyRole {
			if s.hasSubGen(k, sl, r) {
				roles |= 1 << i
			}
		}
		if roles == 0 {
			continue
		}
		next := reach
		for set := 0; set < 16; set++ {
			if reach&(1<<set) == 0 {
				continue
			}
			for i := 0; i < 4; i++ {
				if roles&(1<<i) != 0 && set&(1<<i) == 0 {
					next |= 1 << (set | 1<<i)
				}
			}
		}
		reach = next
	}
	best := 0
	for set := 0; set < 16; set++ {
		if reach&(1<<set) != 0 {
			best = max(best, bits.OnesCount8(uint8(set)))
		}
	}
	return best
}
