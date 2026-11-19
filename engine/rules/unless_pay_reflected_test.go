package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 106.1 / 601.2g: an untapped Fellwar Stone beside an opponent's Mountain
// taps for {R}, so a red unless cost (Pact of the Titan's {4}{R}) is payable
// from it. The unless window census once listed the Stone as its colourless
// stand-in, which left the {R} pip unreachable and the pay option unoffered.
func TestUnlessCostPayableThroughReflectedStone(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := handEngine(t, mustCorpusCard(t, reg, "Fellwar Stone"))
	e.G.Players[0].Pool = state.Mana{}
	stone := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fellwar Stone"))
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Mountain"))

	if o := e.G.Obj(stone); o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Fellwar Stone must be an untapped permanent, got %+v", o)
	}
	units := e.unlessWindowUnits(0)
	if len(units) != 1 || len(units[0].Alts) != 1 || units[0].Alts[0].Counts[state.MR] != 1 {
		t.Fatalf("precondition: the unless census must price the Stone as {R}, got %+v", units)
	}
	if !e.UnlessCostPayable(0, "R") {
		t.Fatal("an untapped Fellwar Stone reflecting a Mountain did not make a {R} unless cost payable")
	}
	if e.UnlessCostPayable(0, "G") {
		t.Fatal("Fellwar Stone reflecting only a Mountain made {G} payable")
	}
}
