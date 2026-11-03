package rules

// cda_power_counts_colours_test.go pins that a power-defining ability counting
// the colours among permanents reads those colours through layer 5 only (CR
// 613.1e, 613.4): it neither recurses through layer 7 nor reads printed
// colours. Squawkroaster's power is the number of colours among permanents
// its controller has; Transguild Courier is all five by its own ability.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestPowerCountingColoursReadsLayerFiveWithoutRecursing(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Squawkroaster"), lookup(t, reg, "Squawkroaster"), lookup(t, reg, "Transguild Courier")}, []*cards.Card{})

	roaster := cdaMoveTo(t, e, 0, "Squawkroaster", state.ZBattlefield)
	if got := e.Power(roaster); got != 1 {
		t.Fatalf("Squawkroaster alone: power = %d, want 1 (red only)", got)
	}
	cdaMoveTo(t, e, 0, "Transguild Courier", state.ZBattlefield)
	// A second such creature: each one's count reads the other's colours, which
	// must not send either back through layer 7.
	var twin state.ObjID
	for _, id := range append(e.G.Zone(state.ZHand, 0), e.G.Zone(state.ZLibrary, 0)...) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Squawkroaster" && id != roaster {
			twin = id
			break
		}
	}
	if twin == 0 {
		t.Fatal("no second Squawkroaster")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: twin, From: e.G.Obj(twin).Zone, To: state.ZBattlefield})
	e.pending = nil
	if got := e.Power(twin); got != 5 {
		t.Fatalf("second Squawkroaster: power = %d, want 5", got)
	}
	if got := e.Power(roaster); got != 5 {
		t.Fatalf("Squawkroaster beside an all-colours permanent: power = %d, want 5", got)
	}
}
