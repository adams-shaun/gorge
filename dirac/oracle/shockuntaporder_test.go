package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// A shock land entering under Warp Speed's "lands you control enter untapped"
// is a CR 616.1 competition over the tapped bit: its body is a tap behind the
// pay-life gate. gorge poses the order, then the election whichever body goes
// first; declining taps the land only when the shock body is applied last. The
// kernel poses the same two asks, counts the land play once, and a declined
// election taps nothing when the untap came last.
func TestShockLandUnderEntersUntappedOrder(t *testing.T) {
	t.Parallel()
	orders := 0
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, Check: func(ref *GorgeSide, _ Side) *Divergence {
		if d := ref.E.Pending(); d != nil && d.Kind == decision.KReplacement {
			if o := ref.E.G.Obj(d.Source); o != nil && o.Face() != nil && o.Face().IsLand() {
				orders++
			}
		}
		return nil
	}}
	synthPairsMulligans(t, "std-trk", rc, 0, []int{13, 108, 151}, nil)
	if orders == 0 {
		t.Fatal("no land's entry-order ask was posed: the pairs no longer reach the competition")
	}
}
