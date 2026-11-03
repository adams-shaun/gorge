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
	synthPairsMulligans(t, "std-trk", rc, 0, []int{131}, nil)
	if orders == 0 {
		t.Fatal("no land's entry-order ask was posed: the pairs no longer reach the competition")
	}
}

// Spock, Voice of Reason draws a card for each nontoken permanent its
// controller has that entered this turn. gorge read the nonToken property as
// unknown and drew none, the kernel drew for each.
func TestNonTokenEnteredThisTurnCount(t *testing.T) {
	t.Parallel()
	synthPairsMulligans(t, "std-trc", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 0, []int{59, 152, 177}, nil)
}

// U.S.S. Voyager's "put those land cards onto the battlefield tapped" with The
// Unicomplex among them: the land enters tapped (CR 614.12), so its "when this
// land enters untapped" charm is never put on the stack. gorge checked the
// trigger between the move and the tap and posed it.
func TestDigUntilTappedLandSkipsEntersUntapped(t *testing.T) {
	t.Parallel()
	synthPairsMulligans(t, "std-trk", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 0, []int{50, 55}, nil)
}
