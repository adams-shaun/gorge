package agent_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// TestRebuildRestoresAbilityFacts: the facts the bot ranks an "ability" option
// by are server-side, so x_gorge_view_v1 carries them beside the decision.
func TestRebuildRestoresAbilityFacts(t *testing.T) {
	l := &decision.Loyalty{Delta: -2}
	_, d := agent.Rebuild(xview.Payload{
		Decision: decision.Decision{Options: []decision.Option{{Kind: "ability"}}},
		Facts:    xview.Facts{Options: []xview.OptionFacts{{Loyalty: l, SelfSkipTurns: 1, ForeignSource: true, GrantStatics: []string{"s"}}}},
	})
	o := d.Options[0]
	if o.Loyalty != l || o.SelfSkipTurns != 1 || !o.ForeignSource || len(o.GrantStatics) != 1 {
		t.Fatalf("restored option %+v", o)
	}
}
