package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestQuickDrawSweepsTheTargetOpponentsCreatures: the sub's own opponent target
// names whose creatures lose first strike, past the spell's creature target
// (CR 608.2b, 611.2c).
func TestQuickDrawSweepsTheTargetOpponentsCreatures(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	knight := lookup(t, reg, "Youthful Knight")
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Quick Draw"), knight}, []*cards.Card{knight})
	mine := moveByName(t, e, 0, "Youthful Knight", state.ZBattlefield)
	theirs := moveByName(t, e, 1, "Youthful Knight", state.ZBattlefield)
	id := moveByName(t, e, 0, "Quick Draw", state.ZHand)
	addMana(t, e, 0, "R")
	submitChoices(t, e, findOption(e.Pending(), "cast", id))
	drainSetMechanic(t, e, func(d *decision.Decision) []int {
		for _, o := range d.Options {
			if o.Obj == mine || o.Kind == "player" && o.Player == 1 {
				return []int{o.Index}
			}
		}
		return firstOption(d)
	})
	if !e.HasKeyword(mine, "First Strike") {
		t.Error("the caster's creature lost first strike")
	}
	if e.HasKeyword(theirs, "First Strike") {
		t.Error("the target opponent's creature kept first strike")
	}
}

// TestInvokeJusticeDistributesAmongTheTargetPlayersCreatures: the link after the
// one that named the player still reads that player, past the root's card target
// (CR 608.2b).
func TestInvokeJusticeDistributesAmongTheTargetPlayersCreatures(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Invoke Justice"), bear}, []*cards.Card{bear})
	moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	theirs := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	id := moveByName(t, e, 0, "Invoke Justice", state.ZHand)
	addMana(t, e, 0, "WWWWW")
	submitChoices(t, e, findOption(e.Pending(), "cast", id))
	drainSetMechanic(t, e, func(d *decision.Decision) []int {
		for _, o := range d.Options {
			if o.Obj == theirs || o.Kind == "player" && o.Player == 1 {
				return []int{o.Index}
			}
		}
		return firstOption(d)
	})
	if got := e.G.Obj(theirs).Counter("P1P1"); got != 4 {
		t.Fatalf("the target player's creature has %d +1/+1 counters, want 4", got)
	}
}
