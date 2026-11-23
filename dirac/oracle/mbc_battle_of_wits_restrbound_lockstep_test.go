package oracle

import "testing"

// A plain cast the payment-restricted potential bound cannot afford is no
// offer on the any-mana walk (restrBoundDrops' cast arm, cast_restrbound.go):
// gorge prices a priority cast offer against the bound built for that payment
// (potentialRestrictionAdmits, CR 106.6: a restricted source whose
// RestrictValid$ refuses the payment adds nothing to it), so Jeweled Lotus's
// commander-only mana is not in the bound the {3}{U}{U} Battle of Wits is
// priced against. The kernel's open any-mana walk counted the three mana for
// the cast, so it offered cast(Battle of Wits) at a priority decision gorge
// ended without it (tail-64 pair 20, decision 301, class options options).
func TestMBCBattleOfWitsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "battle_of_wits_restrbound")
}
