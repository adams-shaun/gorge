package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedConspireUnpaidDoesNotTrigger pins CR 702.78a's intervening "if"
// (CR 603.4) on the SYNTHESIZED granted trigger: "when you cast this spell,
// if its conspire cost was paid, copy it". Under Raiding Schemes' grant a
// Lightning Bolt cast without the tap puts no Conspire ability on the stack
// (the ECL Assert Perfection window), and the conspired cast puts exactly one
// that copies the spell once.
func TestGrantedConspireUnpaidDoesNotTrigger(t *testing.T) {
	t.Parallel()
	t.Run("declined", func(t *testing.T) {
		t.Parallel()
		e, cfg, reg := conspireEngine(t, "Lightning Bolt")
		seedBattlefield(t, e, reg, "Goblin Piker")
		seedBattlefield(t, e, reg, "Goblin Piker")
		seedBattlefield(t, e, reg, "Raiding Schemes")
		bolt := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
		addMana(t, e, 0, "R")
		// Precondition: the grant is live, so the conspired cast is offered
		// and the plain cast is a real decline of it.
		castOptMode(t, e.Pending().Options, bolt, "conspired")
		opt := castOptMode(t, e.Pending().Options, bolt, "")
		submitChoices(t, e, opt.Index)
		chooseTargetPlayer(t, e, 1)
		if n := len(e.G.Stack); n != 1 {
			t.Fatalf("stack after an unpaid granted cast holds %d objects, want only the spell", n)
		}
		passUntilStackEmpty(t, e, 40)
		if n := triggersPushedFrom(e, bolt); n != 0 {
			t.Fatalf("an unpaid granted conspire cast pushed %d triggers, want 0", n)
		}
		if n := copyCount(e, bolt); n != 0 {
			t.Fatalf("an unpaid granted conspire cast made %d copies, want 0", n)
		}
		if life := e.G.Players[1].Life; life != 17 {
			t.Fatalf("opponent life %d, want 17", life)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("paid", func(t *testing.T) {
		t.Parallel()
		e, cfg, reg := conspireEngine(t, "Lightning Bolt")
		seedBattlefield(t, e, reg, "Goblin Piker")
		seedBattlefield(t, e, reg, "Goblin Piker")
		seedBattlefield(t, e, reg, "Goblin Piker")
		seedBattlefield(t, e, reg, "Raiding Schemes")
		bolt := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
		addMana(t, e, 0, "R")
		opt := castOptMode(t, e.Pending().Options, bolt, "conspired")
		submitChoices(t, e, opt.Index)
		bf := e.G.Zone(state.ZBattlefield, 0)
		chooseConspire(t, e, bf[0], bf[1])
		chooseTargetPlayer(t, e, 1)
		passUntilStackEmpty(t, e, 40)
		if n := triggersPushedFrom(e, bolt); n != 1 {
			t.Fatalf("a paid granted conspire cast pushed %d triggers, want 1", n)
		}
		if n := copyCount(e, bolt); n != 1 {
			t.Fatalf("a paid granted conspire cast made %d copies, want 1", n)
		}
		if life := e.G.Players[1].Life; life != 14 {
			t.Fatalf("opponent life %d, want 14", life)
		}
		replayCheck(t, e, cfg)
	})
}
