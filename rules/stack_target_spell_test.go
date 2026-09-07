package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

const manaLeakSpellSrc = "Name:Mana Leak\nManaCost:U\nTypes:Instant\n" +
	"A:SP$ Counter | TargetType$ Spell | TgtPrompt$ Select target spell | ValidTgts$ Card | SpellDescription$ Counter target spell.\nOracle:x\n"

// targetSpellFixture builds the scenario the brief names: a real-corpus-shaped
// counterspell (TargetType$ Spell + ValidTgts$ Card, no TgtZone$) in seat 0's
// hand, a creature spell in seat 0's hand, and an opponent-controlled
// battlefield creature -- the "Gurmag Angler #304" a broken askTarget used to
// offer for the prompt "Select target spell". It returns the engine plus the
// Mana Leak, Bear-spell and battlefield-creature object IDs.
func targetSpellFixture(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	gurmagSrc := "Name:Gurmag\nManaCost:4 B\nTypes:Creature Zombie\nPT:5/5\nOracle:x\n"
	e := handEngine(t, card(t, manaLeakSpellSrc), card(t, bearSrc))
	g := e.G.AddObject(card(t, gurmagSrc), 1)
	g.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{g.ID})

	var leakID, bearID state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		switch e.G.Obj(id).Face().Name {
		case "Mana Leak":
			leakID = id
		case "Bear":
			bearID = id
		}
	}
	if leakID == 0 || bearID == 0 {
		t.Fatalf("fixture hand missing cards: leak=%d bear=%d", leakID, bearID)
	}
	return e, leakID, bearID, g.ID
}

// passToCast submits "pass" on each priority decision until the pending seat
// can take the given cast option, returning that option's index.
func passToCast(t *testing.T, e *Engine, castObj state.ObjID) int {
	t.Helper()
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("expected a priority decision while seeking cast of %d, got %+v", castObj, d)
		}
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == castObj {
				return o.Index
			}
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority decision with no pass and no cast of %d: %+v", castObj, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	t.Fatalf("never got to cast of %d", castObj)
	return -1
}

// TestTargetTypeSpellOffersOnlyStackObjectsAndCounters is the test the suite
// was blind to: it goes THROUGH cast/askTarget with a real-corpus-shaped
// counterspell (TargetType$ Spell + ValidTgts$ Card, no TgtZone$) and asserts
// that (1) a battlefield permanent is NOT offered for the "Select target
// spell" prompt, (2) a spell on the stack IS offered, and (3) the
// counterspell actually counters it end to end -- the targeted spell reaches
// its owner's graveyard and its creature never arrives on the battlefield.
func TestTargetTypeSpellOffersOnlyStackObjectsAndCounters(t *testing.T) {
	e, leakID, bearID, gurmagID := targetSpellFixture(t)
	e.G.Players[0].Pool[state.MU] = 5
	e.G.Players[0].Pool[state.MG] = 5
	e.askPriority(0)

	// Put a creature spell on the stack by casting Bear (it has no targets).
	submitChoices(t, e, passToCast(t, e, bearID))
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the Bear spell", e.G.Stack)
	}
	targetSpell := e.G.Stack[0]

	// Cast Mana Leak in response; its askTarget now fires a KTarget decision.
	submitChoices(t, e, passToCast(t, e, leakID))
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}

	// 1. The opponent's battlefield creature is NOT offered for "Select
	// target spell".
	for _, o := range d.Options {
		if o.Obj == gurmagID {
			t.Fatalf("battlefield permanent %d offered for 'Select target spell': %+v", gurmagID, d.Options)
		}
	}
	// 2. The spell on the stack IS offered.
	spellIdx := -1
	for _, o := range d.Options {
		if o.Obj == targetSpell {
			spellIdx = o.Index
		}
	}
	if spellIdx < 0 {
		t.Fatalf("spell on the stack not offered among %+v", d.Options)
	}

	// 3. Choose the stack spell and let everything resolve: the Bear is
	// countered, reaches its owner's graveyard, and never becomes a permanent,
	// while the battlefield creature is left alone.
	submitChoices(t, e, spellIdx)
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		castFirst(t, e, "pass")
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack did not empty: %v", e.G.Stack)
	}
	if got := e.G.Obj(targetSpell).Zone; got != state.ZGraveyard {
		t.Fatalf("countered spell went to %s, want graveyard", got)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != 0 {
		t.Fatalf("countered Bear spell resolved onto the battlefield: %v", e.G.Zone(state.ZBattlefield, 0))
	}
	if got := e.G.Obj(gurmagID).Zone; got != state.ZBattlefield {
		t.Fatalf("battlefield creature moved to %s", got)
	}
}
