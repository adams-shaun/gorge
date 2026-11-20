package oracle

import "testing"

// CR 601.2c precedes CR 601.2h: Dwarven Miner's "{2}{R}, {T}: Destroy target
// nonbasic land" may target Peat Bog and then pay its own cost by tapping that
// Peat Bog ("{T}, Remove a depletion counter: Add {B}{B}"), which sacrifices it
// once the last counter is gone. Gorge's in-play planner never prices that
// fixed counter removal, and its scripted prefix is refused when the tap would
// consume the play's only target, so gorge withholds the activation. The
// kernel priced it through a prefix that tapped the Peat Bog before the play,
// leaving the offered activation with no legal target; choosing it panicked at
// decision 187 ("spell/ability without a legal target"). The prefix search now
// applies the same post-prefix target census to an activated ability
// (mana_grantcost.go playTargetsFeasible), so both sides withhold the play
// (dr-mbc-bbbb pair 59, decision 186).
func TestMBCDwarvenMinerReprosLockstep(t *testing.T) {
	stdReproNamed(t, "depletion_land_target_paid_by_own_mana")
}
