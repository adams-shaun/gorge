package rules

// CR 700.2d / CR 115.3: a mode chosen more than once is a separate instance
// with its own targets. Season of the Burrow is the budgeted CanRepeatModes$
// corpus carrier whose exile mode can be chosen twice; both instances must
// declare and choose a target, and both targets must be exiled.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestRepeatedModeTargetsPerInstance(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	burrow := lookup(t, reg, "Season of the Burrow")
	bear := lookup(t, reg, "Grizzly Bears")
	lion := lookup(t, reg, "Savannah Lions")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{burrow, bear, lion}, nil)

	var burrowID state.ObjID
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(cand); o != nil && o.Face() != nil && o.Face().Name == "Season of the Burrow" {
			burrowID = cand
		}
	}
	if burrowID == 0 {
		burrowID = moveByName(t, e, 0, "Season of the Burrow", state.ZHand)
	}
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	lionID := moveByName(t, e, 0, "Savannah Lions", state.ZBattlefield)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears zone = %+v, want battlefield", o)
	}
	if o := e.G.Obj(lionID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Savannah Lions zone = %+v, want battlefield", o)
	}

	addMana(t, e, 0, "WWWWW")
	d := castFixture(t, e, burrowID, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the cast-time KModes announcement, got %+v", d)
	}
	exile := -1
	for _, o := range d.Options {
		if o.Value == 2 {
			exile = o.Index
		}
	}
	if exile < 0 {
		t.Fatalf("exile mode not priced 2: %+v", d.Options)
	}
	// Two instances of the exile mode ({P}{P} each, within the {P} five budget).
	submitChoices(t, e, exile, exile)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("pending = %+v, want one target slot per repeated-mode instance", d)
	}
	bearIdx, lionIdx := -1, -1
	for _, o := range d.Options {
		switch {
		case o.Obj == bearID && o.Group == "charm-mode-0":
			bearIdx = o.Index
		case o.Obj == lionID && o.Group == "charm-mode-1":
			lionIdx = o.Index
		}
	}
	if bearIdx < 0 || lionIdx < 0 {
		t.Fatalf("target ask does not offer a target per instance: bear %d lion %d in %+v",
			bearIdx, lionIdx, d.Options)
	}
	submitChoices(t, e, bearIdx, lionIdx)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Grizzly Bears zone = %+v, want exile (first instance's target)", o)
	}
	if o := e.G.Obj(lionID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Savannah Lions zone = %+v, want exile (second instance's target)", o)
	}
	if o := e.G.Obj(burrowID); o == nil || o.Zone != state.ZGraveyard {
		t.Errorf("Season of the Burrow resolved to %+v, want graveyard", o)
	}
	replayCheck(t, e, cfg)
}
