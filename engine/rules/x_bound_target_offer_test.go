package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// xBoundOffered reports whether a kind option names id: an X-priced action whose
// target reads the X, on a board where only the population can rule it out.
func xBoundOffered(e *Engine, kind string, id state.ObjID) bool {
	e.priorityRound()
	for _, o := range e.Pending().Options {
		if o.Kind == kind && o.Obj == id {
			return true
		}
	}
	return false
}

// TestXBoundTargetNeedsACandidate pins CR 601.2c and 602.2b: a target bounded by
// an X not yet announced still needs an object the rest of its description fits.
// No X can make a legal target out of an empty population.
func TestXBoundTargetNeedsACandidate(t *testing.T) {
	t.Parallel()
	const saga = "Name:Old Tale\nManaCost:2\nTypes:Enchantment Saga\nOracle:x\n"
	const bear = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	t.Run("an activated ability", func(t *testing.T) {
		t.Parallel()
		const summoner = "Name:Summoner\nTypes:Creature Human\nPT:1/1\n" +
			"A:AB$ ChangeZone | Cost$ X T | ValidTgts$ Saga.YouOwn+cmcEQX | TgtPrompt$ Pick a tale | Origin$ Graveyard | Destination$ Battlefield | SpellDescription$ Bring back a tale.\n" +
			"SVar:X:Count$xPaid\nOracle:x\n"
		e := handEngine(t)
		id := onBoardReady(t, e, 0, summoner)
		e.G.Players[0].Pool[state.MC] = 4
		if xBoundOffered(e, "ability", id) {
			t.Fatal("offered with no tale in the graveyard")
		}
		tale := e.G.AddObject(card(t, saga), 0)
		tale.Zone = state.ZGraveyard
		e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{tale.ID})
		if !xBoundOffered(e, "ability", id) {
			t.Fatal("not offered with a tale in the graveyard")
		}
	})
	t.Run("a spell", func(t *testing.T) {
		t.Parallel()
		const shrink = "Name:Shrink Ray\nManaCost:X\nTypes:Sorcery\n" +
			"A:SP$ Destroy | ValidTgts$ Creature.cmcLEX | TgtPrompt$ Pick a creature | SpellDescription$ Destroy a small one.\n" +
			"SVar:X:Count$xPaid\nOracle:x\n"
		e := handEngine(t, card(t, shrink))
		e.G.Players[0].Pool[state.MC] = 4
		id := e.G.Zone(state.ZHand, 0)[0]
		if xBoundOffered(e, "cast", id) {
			t.Fatal("offered with no creature on the battlefield")
		}
		onBoard(t, e, 1, bear)
		if !xBoundOffered(e, "cast", id) {
			t.Fatal("not offered with a creature on the battlefield")
		}
	})
}

// TestXBoundTargetNeedsAnAffordableAnnouncement pins CR 601.2b, 601.2c and 601.2h:
// an announcement must both reach a target and pay, so the action is offered only
// when the mana within reach, floating or on untapped sources, buys such an X.
func TestXBoundTargetNeedsAnAffordableAnnouncement(t *testing.T) {
	t.Parallel()
	const saga = "Name:Old Tale\nManaCost:2\nTypes:Enchantment Saga\nOracle:x\n"
	const bear = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const forest = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
	const summoner = "Name:Summoner\nTypes:Creature Human\nPT:1/1\n" +
		"A:AB$ ChangeZone | Cost$ X T | ValidTgts$ Saga.YouOwn+cmcEQX | TgtPrompt$ Pick a tale | Origin$ Graveyard | Destination$ Battlefield | SpellDescription$ Bring back a tale.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const shrink = "Name:Shrink Ray\nManaCost:X\nTypes:Sorcery\n" +
		"A:SP$ Destroy | ValidTgts$ Creature.cmcLEX | TgtPrompt$ Pick a creature | SpellDescription$ Destroy a small one.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	newSummoner := func(t *testing.T) (*Engine, state.ObjID) {
		e := handEngine(t)
		id := onBoardReady(t, e, 0, summoner)
		tale := e.G.AddObject(card(t, saga), 0)
		tale.Zone = state.ZGraveyard
		e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{tale.ID})
		return e, id
	}
	t.Run("an ability, one mana floating", func(t *testing.T) {
		t.Parallel()
		e, id := newSummoner(t)
		e.G.Players[0].Pool[state.MC] = 1
		if xBoundOffered(e, "ability", id) {
			t.Fatal("offered with no X that pays for the tale")
		}
	})
	t.Run("an ability, the X floating", func(t *testing.T) {
		t.Parallel()
		e, id := newSummoner(t)
		e.G.Players[0].Pool[state.MC] = 2
		if !xBoundOffered(e, "ability", id) {
			t.Fatal("not offered with the X floating")
		}
	})
	t.Run("an ability, the X on untapped lands", func(t *testing.T) {
		t.Parallel()
		e, id := newSummoner(t)
		onBoardReady(t, e, 0, forest)
		onBoardReady(t, e, 0, forest)
		if !xBoundOffered(e, "ability", id) {
			t.Fatal("not offered with the X on untapped lands")
		}
	})
	t.Run("a spell", func(t *testing.T) {
		t.Parallel()
		e := handEngine(t, card(t, shrink))
		id := e.G.Zone(state.ZHand, 0)[0]
		onBoard(t, e, 1, bear)
		e.G.Players[0].Pool[state.MC] = 1
		if xBoundOffered(e, "cast", id) {
			t.Fatal("offered with no X that reaches the creature")
		}
		e.G.Players[0].Pool[state.MC] = 2
		if !xBoundOffered(e, "cast", id) {
			t.Fatal("not offered with the X floating")
		}
	})
}
