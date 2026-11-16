package oracle

import "testing"

// CR 106.6 with 106.4: Dalakos, Crafter of Wonders' {C}{C} may be spent only
// on artifact spells and abilities of artifacts. Tapped to pay Bonesplitter's
// equip {1}, the {C} left over still floats under that restriction, so
// Blood Rites' {1}{R} (an enchantment's ability) cannot use it with a lone
// Mountain. The kernel's sole restricted two-mana ability put its surplus in
// the ordinary pool, so it offered Blood Rites' activation and gorge did not
// (first difference: options at a priority decision, decision 432 of
// tail-41 pair 421).
func TestMBCBloodRitesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_overpay_floats_restricted")
}

// CR 106.6 with CR 106.4: Vedalken Engineer's {T} adds two mana of one colour
// that may only cast artifact spells (or activate artifact abilities). Tapped
// for a {W} Portable Hole it leaves one restricted {W} floating, which cannot
// pay for Sunfire Balm's {2}{W} cast or its {1}{W} cycling. The kernel once
// floated the overpay as ordinary mana (a sole restricted ability of two mana
// kept a plain unit) and offered both plays that gorge withholds (decision 120
// of tail-17 pair 7). The recording is pooled custom: the repro's own decks
// must not depend on a synthetic pool's membership.
func TestMBCSunfireBalmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_two_mana_overpay_floats_restricted")
}
