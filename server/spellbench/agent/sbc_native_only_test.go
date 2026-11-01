package agent_test

// A dig to a non-library primary destination (Matter
// Reshaper's battlefield-or-hand shape) marks each partition candidate with
// Inside, because neither "battlefield" nor "hand" follows the legacy
// top/hand destination convention.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

func TestPickUsesTheDestInsideHint(t *testing.T) {
	none := func(decision.Decision) decision.Intent { return decision.Intent{} }
	inside, outside := true, false
	sems := []map[string]any{{"kind": "arrange_card"}, {"kind": "arrange_card"}}
	p := xview.Payload{Ops: []mapping.NativeOp{
		{Op: "dest", Option: 0, List: "battlefield", Inside: &inside},
		{Op: "dest", Option: 0, List: "hand", Inside: &outside},
	}}
	// The bot selected the card: it goes to the primary (battlefield).
	pl := agent.NewPlan(1, decision.Intent{Choices: []int{0}})
	if i, miss := agent.Pick(p, sems, pl, none); i != 0 || miss != "" {
		t.Fatalf("selected card went to candidate %d (%q), want the battlefield", i, miss)
	}
	// The bot left it: the remainder goes to the second destination (hand).
	pl = agent.NewPlan(1, decision.Intent{})
	if i, miss := agent.Pick(p, sems, pl, none); i != 1 || miss != "" {
		t.Fatalf("declined card went to candidate %d (%q), want the hand", i, miss)
	}
}
