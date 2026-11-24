package kgen

import "testing"

// TestRestrictedManaFeeAnyAloneRefuses: one mana of any colour is a fee
// production the kernel carries only as the fee alternative of a free
// restricted ability under the same restriction (Hydro-Channeler,
// restrictedFreeFeePair). A source whose only mana ability is the restricted
// "{1}, {T}: Add one mana of any color" has no free unit for gorge's census to
// list, so it stays refused.
func TestRestrictedManaFeeAnyAloneRefuses(t *testing.T) {
	const src = "Name:Probe Any-Fee Relic\nManaCost:2\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ 1 T | Produced$ Any | RestrictValid$ Spell.Instant,Spell.Sorcery | SpellDescription$ Add one mana of any color, for instant and sorcery spells only.\n" +
		"Oracle:{1}, {T}: Add one mana of any color. Spend this mana only to cast an instant or sorcery spell.\n"
	lc := lowerSynth(t, src)
	var mana int
	for _, a := range lc.Abilities {
		if a.Mana != nil && a.Mana.Restrict != nil && a.Mana.Colors == "WUBRG" && a.Cost.Mana.Generic == 1 {
			mana++
		}
	}
	if mana != 1 {
		t.Fatalf("want the one restricted any-colour fee ability parsed, got %d of %+v", mana, lc.Abilities)
	}
	if len(lc.Refusals) == 0 {
		t.Fatalf("a lone restricted any-colour fee source lowered, want a refusal")
	}
	t.Logf("refusal: %s", lc.Refusals[0].Reason)
}
