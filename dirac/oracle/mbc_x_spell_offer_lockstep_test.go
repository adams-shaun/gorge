package oracle

import "testing"

// CR 601.2g / CR 605.3a: mana abilities with their own mana cost ("{1}, {T}: Add one mana of any
// color", Artifact Unknown Shores) may be activated while paying for a spell. Killing Glare
// ({X}{B}) is payable with the lone untapped sources Forests, Baldur's Gate and the Shores, but
// only one engine offered the cast (decision 566, tail-51 pair 3).
func TestMBCKillingGlareReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_spell_offer_fee_any_color")
}
