package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDiscardedCostValueRef casts the real corpus Grab the Prize paying its
// "Discard<1/Card>" additional cost with a nonland and then a land card. The
// damage SVar reads `Discarded$Valid Card.nonLand/Times.2`, so the nonland
// discard deals 2 to the opponent and the land discard deals none; both draw
// two.
func TestDiscardedCostValueRef(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		discard string
		damage  int32
	}{
		{"nonland", "Grizzly Bears", 2},
		{"land", "Forest", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			e, cfg := mordorEngine(t, reg, 4201, "Grab the Prize", "Grizzly Bears", "Forest")
			grab := searchMoveByName(t, e, "Grab the Prize", state.ZHand)
			disc := searchMoveByName(t, e, tc.discard, state.ZHand)
			addMana(t, e, 0, "1R")
			life, draws := e.G.Players[1].Life, countDrawFor(e, 0)
			castNoDrain(t, e, grab)
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "discard" {
				t.Fatalf("cost discard ask = %+v", d)
			}
			submitObj(t, e, "discard", disc)
			passUntilStackEmpty(t, e, 30)
			if o := e.G.Obj(disc); o == nil || o.Zone != state.ZGraveyard {
				t.Fatalf("discarded card not in the graveyard: %+v", o)
			}
			if got := countDrawFor(e, 0) - draws; got != 2 {
				t.Fatalf("drew %d, want 2", got)
			}
			if got := life - e.G.Players[1].Life; got != tc.damage {
				t.Fatalf("opponent lost %d life, want %d", got, tc.damage)
			}
			replayCheck(t, e, cfg)
		})
	}
}
