package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// flashbackReachable: the flashback cast of card c (object o) at cost fc is one
// gorge's priority list can carry. Its additional cost's permanents are held
// back from the mana (flashbackPartsDisjoint), and on the autopay surface a
// cost payable only with a mana ability an effect grants (Abundant Growth's
// "{T}: Add one mana of any color" on the only land making {R}) is left out:
// gorge prices a flashback cast through its potential planner, whose census
// holds printed and intrinsic abilities only (rules/pay PaymentAbility), so it
// answers census_incomplete and drops the play, while the floating pool alone
// pays a cast it lists as offered (FeatPoolPays).
func (s *State) flashbackReachable(k *prep, p uint8, c CardID, o Obj, fc kop.ManaCost) bool {
	if !s.flashbackPartsDisjoint(k, p, c, fc) {
		return false
	}
	if s.cfg&cfgGorgeAutopay == 0 || k.gen == nil || len(k.gen.grantPC) == 0 || s.FeatPoolPays(p, o, 2) {
		return true
	}
	return s.affordsUngrantedCast(k, p, &fc, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)})
}
