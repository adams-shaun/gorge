package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// emergedWall is a 0/4 for {W}: mana value 1 and toughness 4, so Adipose
// Offspring's emerge cost {5}{W} reduces to {4}{W} and the head reads 4, a
// number neither the sacrificed creature's mana value nor its power.
const emergedWall = "Name:Emerged Wall\nManaCost:W\nTypes:Creature Wall\nPT:0/4\nOracle:x\n"

func emergedTokenCount(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			n++
		}
	}
	return n
}

// TestEmergedHeadReadsSacrificedToughness: Adipose Offspring's ETB creates X
// tokens, X the toughness of the creature sacrificed for its emerge cost
// (Count$Emerged.T.1 with SVar T Emerged$CardToughness), and one token when
// it was cast for its mana cost.
func TestEmergedHeadReadsSacrificedToughness(t *testing.T) {
	t.Parallel()
	t.Run("emerged", func(t *testing.T) {
		t.Parallel()
		e, cfg, _ := altCostEngine(t, 4242, []string{"Adipose Offspring"}, []string{emergedWall}, nil)
		id := findCardObj(t, e, 0, "Adipose Offspring", state.ZHand)
		wall := findCardObj(t, e, 0, "Emerged Wall", state.ZBattlefield)
		if tough := e.Toughness(wall); tough != 4 {
			t.Fatalf("precondition: sacrificed creature toughness %d, want 4", tough)
		}
		if before := emergedTokenCount(e, 0); before != 0 {
			t.Fatalf("precondition: %d tokens before the cast", before)
		}
		addMana(t, e, 0, "WCCCCC")
		castEmergeSacrificing(t, e, id, wall)
		if o := e.G.Obj(wall); o.Zone != state.ZGraveyard {
			t.Fatalf("sacrificed creature in %s, want graveyard", o.Zone)
		}
		if got := emergedTokenCount(e, 0); got != 4 {
			t.Fatalf("emerged cast made %d tokens, want 4 (the sacrificed creature's toughness)", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("plain cast", func(t *testing.T) {
		t.Parallel()
		e, cfg, _ := altCostEngine(t, 4242, []string{"Adipose Offspring"}, []string{emergedWall}, nil)
		id := findCardObj(t, e, 0, "Adipose Offspring", state.ZHand)
		findCardObj(t, e, 0, "Emerged Wall", state.ZBattlefield)
		addMana(t, e, 0, "WCCC")
		submitChoices(t, e, castModeOption(t, e, id, ""))
		for i := 0; i < 20 && emergedTokenCount(e, 0) == 0; i++ {
			d := e.Pending()
			if d == nil {
				t.Fatal("plain cast stalled with no pending decision")
			}
			if d.Kind == decision.KChoose {
				for _, o := range d.Options {
					if o.Kind == "done" {
						submitChoices(t, e, o.Index)
					}
				}
				continue
			}
			passOnceP(t, e)
		}
		if got := emergedTokenCount(e, 0); got != 1 {
			t.Fatalf("plain cast made %d tokens, want 1", got)
		}
		replayCheck(t, e, cfg)
	})
}
