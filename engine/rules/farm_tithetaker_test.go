package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.pool` (xe:RAND:1:14:w13 and 16 sibling windows; Forge
// keeps one unit, gorge spends it): Tithe Taker's "During your turn, spells
// your opponents cast cost {1} more" (S:Mode$ RaiseCost | Activator$ Opponent
// | Condition$ PlayerTurn). CR 109.5: "your" on a static ability means its
// controller, so the tax applies on the Tithe Taker controller's turn, never
// on the caster's own turn. CR 601.2f: only a live cost increase raises the
// total cost. Forge and XMage both leave B in the pool after the untaxed
// Magus cast below.
func TestFarmTitheTaker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	run := func(t *testing.T, scenario string) oracleSnapshotPlayers {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v", res.Fails)
		}
		last := res.Snapshots[len(res.Snapshots)-1]
		return oracleSnapshotPlayers{last.Players[0].Pool, last.Players[1].Pool, len(last.Stack)}
	}
	t.Run("opponent casts on the Tithe Taker controller's turn taxed", func(t *testing.T) {
		got := run(t, `{
		  "name": "farm-tithe-taker-controller-turn",
		  "setup": {"p0": {"battlefield": ["Tithe Taker"]},
		    "p1": {"battlefield": ["Mountain","Mountain"], "hand": ["Lightning Bolt"]}},
		  "steps": [
		    {"op": "pass", "seat": 0},
		    {"op": "activate", "seat": 1, "card": "p1:Mountain", "ability_index": 0},
		    {"op": "activate", "seat": 1, "card": "p1:Mountain#2", "ability_index": 0},
		    {"op": "cast", "seat": 1, "card": "p1:Lightning Bolt", "targets": ["p0"]}
		  ]
		}`)
		if got.stack != 1 {
			t.Fatalf("precondition: Lightning Bolt not on the stack (stack %d)", got.stack)
		}
		if got.p1 != "" {
			t.Fatalf("p1 pool = %q, want empty: Bolt costs {1}{R} on the Tithe Taker controller's turn", got.p1)
		}
	})
	t.Run("opponent casts on its own turn untaxed", func(t *testing.T) {
		got := run(t, `{
		  "name": "farm-tithe-taker-own-turn",
		  "setup": {"p0": {"battlefield": ["Swamp","Plains","Swamp","Plains"], "hand": ["Magus of the Will"]},
		    "p1": {"battlefield": ["Tithe Taker"]}},
		  "steps": [
		    {"op": "activate", "seat": 0, "card": "p0:Swamp", "ability_index": 0},
		    {"op": "activate", "seat": 0, "card": "p0:Plains", "ability_index": 0},
		    {"op": "activate", "seat": 0, "card": "p0:Swamp#2", "ability_index": 0},
		    {"op": "activate", "seat": 0, "card": "p0:Plains#2", "ability_index": 0},
		    {"op": "cast", "seat": 0, "card": "p0:Magus of the Will"}
		  ]
		}`)
		if got.stack != 1 {
			t.Fatalf("precondition: Magus of the Will not on the stack (stack %d)", got.stack)
		}
		if got.p0 != "B" {
			t.Fatalf("p0 pool = %q, want B: Tithe Taker taxes only on its controller's turn (CR 109.5)", got.p0)
		}
	})
}

type oracleSnapshotPlayers struct {
	p0, p1 string
	stack  int
}
