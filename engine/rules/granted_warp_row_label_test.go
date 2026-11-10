package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedWarpRowLabels: when Spirited Simulacrum's printed warp {4} and
// Tannuk's granted {2}{R} are both offered, the two "warped" rows carry
// distinct labels naming each instance's cost, so a client can tell them
// apart without reading AltCostIndex. A single-instance row keeps the bare
// "(warped)" label.
func TestGrantedWarpRowLabels(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 957, []string{"Tannuk, Steadfast Second", "Spirited Simulacrum", "Red Tiger Mechan"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	sim := findCardObj(t, e, 0, "Spirited Simulacrum", state.ZHand)
	mech := findCardObj(t, e, 0, "Red Tiger Mechan", state.ZHand)
	addMana(t, e, 0, "RRRR")
	if got := warpedRows(e, sim); len(got) != 2 {
		t.Fatalf("Simulacrum warped rows with RRRR: %v, want both instances", got)
	}
	labels := map[int]string{}
	var mechLabels []string
	for _, o := range e.Pending().Options {
		if o.Kind != "cast" || o.Mode != "warped" {
			continue
		}
		switch o.Obj {
		case sim:
			labels[o.AltCostIndex] = o.Label
		case mech:
			mechLabels = append(mechLabels, o.Label)
		}
	}
	want := map[int]string{
		0: "Cast Spirited Simulacrum (warped, " + formatCost(ParseCost("4")) + ")",
		1: "Cast Spirited Simulacrum (warped, " + formatCost(ParseCost("2 R")) + ")",
	}
	if labels[0] == labels[1] {
		t.Fatalf("both warped rows share the label %q", labels[0])
	}
	for i, w := range want {
		if labels[i] != w {
			t.Fatalf("warped row %d label %q, want %q", i, labels[i], w)
		}
	}
	if len(mechLabels) != 1 || mechLabels[0] != "Cast Red Tiger Mechan (warped)" {
		t.Fatalf("Red Tiger Mechan warped labels %v, want one bare (warped) row", mechLabels)
	}
}
