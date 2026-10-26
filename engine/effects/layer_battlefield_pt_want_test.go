package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// wantRecorder is a Host whose only method is the layerTablesHost one; it
// records the want set layerTablesFor asks for.
type wantRecorder struct {
	Host
	want LayerTableSet
}

func (w *wantRecorder) LayerTables(want LayerTableSet) LayerTables {
	w.want = want
	return LayerTables{}
}

// TestLayerBattlefieldPTWantBit pins the off-feature cost guard: a body whose
// parameters read no power/toughness never requests the battlefield P/T
// table, and one that does (here also through its SubAbility) always does.
func TestLayerBattlefieldPTWantBit(t *testing.T) {
	t.Parallel()
	asked := func(sa *cards.SA) bool {
		w := &wantRecorder{}
		layerTablesFor(w, sa)
		return w.want&LayerBattlefieldPT != 0
	}
	if asked(&cards.SA{Params: map[string]string{"ValidCards": "Creature.Red", "NumCards": "1"}}) {
		t.Fatal("a body that reads no P/T asks for the battlefield P/T table")
	}
	if !asked(&cards.SA{Params: map[string]string{"ValidCards": "Creature.powerGE4"}}) {
		t.Fatal("a power-reading body must ask for the battlefield P/T table")
	}
	sub := &cards.SA{Params: map[string]string{"Defined": "Targeted.powerLE0"}}
	if !asked(&cards.SA{Params: map[string]string{"NumAtt": "-3"}, Sub: sub}) {
		t.Fatal("a body whose SubAbility reads power must ask for the battlefield P/T table")
	}
}
