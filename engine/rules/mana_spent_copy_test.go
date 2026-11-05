package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGogglesRiderCopiesTheSpellItsManaPaid pins Pyromancer's Goggles'
// TriggersWhenSpent$ rider: its {R} pays a red instant, the rider queues, and
// its Defined$ TriggeredSpellAbility copy resolves to that spell (a StackCopy
// naming it, and the copy on the stack), not to nothing.
func TestGogglesRiderCopiesTheSpellItsManaPaid(t *testing.T) {
	t.Parallel()
	e, _, _ := pathManaGame(t, 93, "Wort, Boggart Auntie", "Pyromancer's Goggles", "Shock")
	gogID := moveToBattlefieldByName(t, e, 0, "Pyromancer's Goggles")
	shockID := moveSeededToHand(t, e, 0, "Shock")
	e.priorityRound()
	activateMana(t, e, gogID)
	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("pool R = %d, want 1 from Goggles", got)
	}
	castSeeded(t, e, shockID)
	for i := 0; i < 40 && e.Pending() != nil; i++ {
		d := e.Pending()
		if d.Kind == decision.KPriority && len(e.G.Stack) > 0 && !riderResolved(e) {
			submitChoices(t, e, passOption(t, d).Index)
			continue
		}
		if d.Kind == decision.KTarget || d.Kind == decision.KChoose {
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		break
	}
	copies := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.StackCopy && ev.Obj == shockID {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("StackCopy events naming Shock = %d, want 1 (the rider's copy of the spell its mana paid for)", copies)
	}
}

func riderResolved(e *Engine) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.StackCopy {
			return true
		}
	}
	return false
}
