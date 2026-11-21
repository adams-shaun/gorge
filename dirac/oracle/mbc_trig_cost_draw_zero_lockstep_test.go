package oracle

import "testing"

// CR 118.6 with the window's own gate (cumulative.go
// triggeredCostComponentsPayable): a trigger's own Draw<X/You> cost is
// payable once its count RESOLVES, zero included -- "you may draw 0 cards;
// if you do, lose 0 life" (Bebop, Skull & Crossbones' Draw<Count$CardCounters.ALL/You>,
// X = 0 on an uncountered Bebop, decision 214 of tail-76-c000 pair 0,
// seed 6751576830972939371) offers pay and decline. An unresolvable X is the
// only decline-only ask (the kgen refusal of a SVar the lowerer cannot read).
// The kernel once priced a resolved zero unpayable and offered decline only.
func TestMBCBebopSkullCrossbonesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trig_cost_draw_zero")
}
