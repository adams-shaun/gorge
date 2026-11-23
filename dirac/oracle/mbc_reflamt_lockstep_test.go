package oracle

import "testing"

// CR 106.1, 119.4: Incubation Druid (three counters, so its {T} adds three mana
// of one type) beside Ifnir Deadlands ({B} costs 1 life), Nomad Stadium ({W}
// deals 1 damage) and a Swamp, the controller at 1 life: Vengeant Vampire
// ({4}{B}{B}) is payable by the Druid's {B}{B}{B} and the other three lands.
// gorge's prefix search runs for every census-incomplete play and colours the
// whole Druid amount; the kernel took the play out because its gaps walk folds
// the Druid as {C} only and withholds Ifnir's painful {B} at one life
// (tail-55 pair 52, decision 487).
//
// Incubation Druid is a planner unit of gorge's evaluated-amount layer now
// (mana_reflamt.go), and its autopay taps the Druid earlier in the game than the
// old prefix search did, so the recorded tail (from decision 442, an attack the
// tapped Druid no longer makes) drifted: the recording is cut at the first
// action gorge no longer offers. The coloured-pip scenario at its end is pinned
// by dirac's TestReflAmtUnitFundsColouredPipsAtOneLife.
func TestMBCVengeantVampireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_source_funds_coloured_pips_at_one_life")
}
