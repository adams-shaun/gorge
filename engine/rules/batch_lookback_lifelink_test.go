package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBatchLookBackTriggerUsesSourceLifelinkLKI pins a trigger whose source
// leaves in the same destruction batch as the creature it watches: Curzon
// Dax, Bound by Blood has lifelink until end of turn and is destroyed by one
// DestroyAll with two other Warriors. Each "another Warrior died" trigger
// looks back to the shared pre-batch board (CR 603.10a), and each resolves
// with Curzon in the graveyard, so its damage reads Curzon's last known
// characteristics, lifelink included (CR 113.7a, 702.15b): two damage, two
// life. The batch moves its members one by one, so a Warrior destroyed after
// Curzon queued its trigger after Curzon's own departure snapshot was taken;
// that trigger must still carry the snapshot, wherever Curzon sits among the
// batch.
func TestBatchLookBackTriggerUsesSourceLifelinkLKI(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const curzon = "Curzon Dax, Bound by Blood"
	for _, board := range [][]string{
		{"Viashino Warrior", curzon, "Borderland Minotaur"},
		{curzon, "Viashino Warrior", "Borderland Minotaur"},
		{"Viashino Warrior", "Borderland Minotaur", curzon},
	} {
		t.Run(board[0]+"/"+board[1]+"/"+board[2], func(t *testing.T) {
			e, cfg, ids := dsBoard(t, reg, board...)
			cid := ids[curzon]
			for _, n := range board {
				if z := e.G.Obj(ids[n]).Zone; z != state.ZBattlefield {
					t.Fatalf("precondition: %s in %s, want battlefield", n, z)
				}
			}
			effects.Resolve(e, &effects.Ctx{Source: cid, Controller: 0},
				&cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{"Defined": "Self", "KW": "Lifelink"}})
			if !e.HasKeyword(cid, "Lifelink") {
				t.Fatal("precondition: Curzon lacks the until-end-of-turn lifelink")
			}
			var land state.ObjID
			for i := range e.G.Objs {
				if o := &e.G.Objs[i]; o.Owner == 0 && o.Face() != nil && o.Face().Name == "Mountain" {
					land = o.ID
					break
				}
			}
			effects.Resolve(e, &effects.Ctx{Source: land, Controller: 0},
				&cards.SA{Kind: "SP", API: "DestroyAll", Params: map[string]string{"ValidCards": "Creature"}})
			for _, n := range board {
				if z := e.G.Obj(ids[n]).Zone; z != state.ZGraveyard {
					t.Fatalf("precondition: %s in %s after DestroyAll, want graveyard", n, z)
				}
			}
			e.Advance()
			for i := 0; i < 40 && e.Pending() != nil && e.Pending().Kind == decision.KTriggerOrder; i++ {
				answerTriggerOrders(t, e)
			}
			drainStack(t, e, 40)
			if got := e.G.Players[1].Life; got != 18 {
				t.Fatalf("opponent life = %d, want 18 (two Warrior triggers, 1 each)", got)
			}
			if got := e.G.Players[0].Life; got != 22 {
				t.Fatalf("Curzon's controller life = %d, want 22: each trigger's damage reads Curzon's lifelink LKI", got)
			}
			if n := countLifeChanges(t, e, 0, 1); n != 2 {
				t.Fatalf("logged %d lifelink LifeChange(0, +1) events, want 2", n)
			}
			replayCheck(t, e, cfg)
		})
	}
}
