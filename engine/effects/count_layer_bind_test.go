package effects

import (
	"github.com/adams-shaun/gorge/engine/state"
	"testing"
)

// layerColorHost is a fakeHost that publishes a layer-5 colour table (the
// Bear is blue, not its printed green) through the layerTablesHost seam, and
// records what it was asked for.
type layerColorHost struct {
	*fakeHost
	bear  state.ObjID
	asked []LayerTableSet
}

func (h *layerColorHost) LayerTables(want LayerTableSet) LayerTables {
	if want != 0 { // the always-published layer-3/4 read asks for none
		h.asked = append(h.asked, want)
	}
	var t LayerTables
	if want&LayerColors != 0 {
		t.DerivedColors = []ObjectColors{{ID: h.bear, Mask: ColorMaskFromLetters("U")}}
	}
	return t
}

// TestCountValidBindsLayerColours pins that a Count$Valid colour filter built
// in a plain Ctx (a cost or mana-amount context, not a CDA that pre-binds the
// colour table) reads the layered colour (CR 613.3), not the printed one: the
// Bear is printed green, made blue by layer 5.
func TestCountValidBindsLayerColours(t *testing.T) {
	g, ids := board(t)
	h := &layerColorHost{fakeHost: &fakeHost{g: g}, bear: ids["myBear"]}
	c := &Ctx{Controller: 0}
	if c.Layers.DerivedColors != nil {
		t.Fatal("precondition: a plain Ctx binds no colour table")
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl+Green"); got != 0 {
		t.Errorf("Creature.YouCtrl+Green = %d, want 0 (the Bear is layered blue)", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl+Blue"); got != 2 {
		t.Errorf("Creature.YouCtrl+Blue = %d, want 2 (layered-blue Bear + the Flier)", got)
	}
	if len(h.asked) == 0 || h.asked[0]&LayerColors == 0 {
		t.Errorf("host asked %v, want a LayerColors request", h.asked)
	}
	// A spec that cannot read a colour asks the host for nothing.
	h.asked = nil
	EvalCount(h, c, "Count$Valid Creature.YouCtrl")
	if len(h.asked) != 0 {
		t.Errorf("a colour-free spec asked the host for %v", h.asked)
	}
}
