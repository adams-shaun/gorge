package oracle

import "testing"

// Drifter il-Dal's upkeep "sacrifice unless you pay {U}" ask beside
// Shimmering Grotto's {1},{T}: any colour: gorge's unless window funds the
// {U} from the filter's mana (CR 605.3b), so the pay arm is a legal answer;
// dirac offered the sacrifice arm only (pool tail-56-c000, pair 2, decision
// 1454).
func TestMBCDrifterIlDalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "drifter_il_dal_unless_filter_modes")
}
