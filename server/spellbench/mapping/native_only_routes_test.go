package mapping_test

// The v2 mapping classes that could fall back to
// the native wire on the legacy 2-seat corpus (choose/exilecost,
// choose/hidden_pick, choose/vote_card), plus the look a hidden-object
// yes/no ask needs.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

func TestNativeOnlyChooseClassesRoute(t *testing.T) {
	cases := []struct {
		kind  string
		want  string
		extra func(*decision.Decision)
	}{
		{"exilecost", "choose/cost", nil},
		{"hidden_pick", "choose/hidden_pick", nil},
		{"vote_card", "choose/vote_card", nil},
	}
	for _, c := range cases {
		d := &decision.Decision{Kind: decision.KChoose, Source: 9,
			Options: []decision.Option{{Kind: c.kind}}}
		if got := mapping.Route(d); got != c.want {
			t.Errorf("Route(%s) = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestExileCostIsACostTargetKind(t *testing.T) {
	if got := mapping.CostKinds["exilecost"]; got != "exile" {
		t.Fatalf("CostKinds[exilecost] = %q, want exile", got)
	}
}
