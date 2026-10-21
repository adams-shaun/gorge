package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 707.10: "If an effect of the copy refers to objects used to pay its
// costs, it uses the objects used to pay the costs of the original spell or
// ability." The rule's own example is a copied Fling, which deals damage
// equal to the power of the creature sacrificed to cast the ORIGINAL Fling.
// All cards are real corpus cards; no Forge script text is committed.

// cr707CastCostPaid casts spell from seat 0's hand, answering its target ask
// with target (an object, or the opponent when target is 0) and any cost
// choice with the option naming paid.
func cr707CastCostPaid(t *testing.T, e *Engine, spell, paid, target state.ObjID) {
	t.Helper()
	submitChoices(t, e, castOptionFor(t, e, spell).Index)
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("CR 707.10 seq %d: no decision while casting", len(e.L.Events))
		}
		switch {
		case d.Kind == decision.KTarget && target == 0:
			submitChoices(t, e, optForPlayer(t, d, 1))
		case d.Kind == decision.KTarget:
			submitChoices(t, e, optForObj(t, d, target))
		case d.Kind == decision.KChoose:
			submitChoices(t, e, optForObj(t, d, paid))
		case d.Kind == decision.KPriority:
			if o := e.G.Obj(spell); o == nil || o.Zone != state.ZStack {
				t.Fatalf("CR 707.10 seq %d: the spell is not on the stack after casting", len(e.L.Events))
			}
			return
		default:
			t.Fatalf("CR 707.10 seq %d: unexpected %s decision while casting", len(e.L.Events), d.Kind)
		}
	}
	t.Fatalf("CR 707.10 seq %d: the cast did not settle", len(e.L.Events))
}

// cr707TwincastAndResolve casts Twincast on spell, keeps the copy's targets
// (the offered inherited target, target or the opponent when target is 0)
// and resolves the stack empty.
func cr707TwincastAndResolve(t *testing.T, e *Engine, twin, spell, target state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "UU")
	submitChoices(t, e, castOptionFor(t, e, twin).Index)
	tt := e.Pending()
	if tt == nil || tt.Kind != decision.KTarget {
		t.Fatalf("CR 707.10 seq %d: Twincast's target ask missing: %+v", len(e.L.Events), tt)
	}
	submitChoices(t, e, optForObj(t, tt, spell))
	for n := 0; n < 40 && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d != nil && d.Kind == decision.KTarget && d.ResumeKind == "copy_targets" {
			if target == 0 {
				submitChoices(t, e, optForPlayer(t, d, 1))
			} else {
				submitChoices(t, e, optForObj(t, d, target))
			}
			continue
		}
		passUntilStackEmpty(t, e, 1)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("CR 707.10 seq %d: the stack did not empty", len(e.L.Events))
	}
}

// TestCR707_10CopiedFlingUsesTheOriginalSacrifice: Fling sacrifices Grizzly
// Bears (power 2) aimed at the opponent, and Twincast copies it. The copy
// uses the creature sacrificed for the original: 2 damage, and the original
// another 2, so the opponent goes to 16 (today the copy deals 0: 18).
func TestCR707_10CopiedFlingUsesTheOriginalSacrifice(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{choiceCorpusCard(t, "Fling"), choiceCorpusCard(t, "Twincast"), choiceCorpusCard(t, "Grizzly Bears")}, nil)
	fling := moveByName(t, e, 0, "Fling", state.ZHand)
	twin := moveByName(t, e, 0, "Twincast", state.ZHand)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	life0 := e.G.Players[1].Life

	addMana(t, e, 0, "RR")
	cr707CastCostPaid(t, e, fling, bears, 0)
	if o := e.G.Obj(bears); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("CR 707.10 Fling seq %d: Grizzly Bears was not sacrificed", len(e.L.Events))
	}
	cr707TwincastAndResolve(t, e, twin, fling, 0)

	if got := e.G.Players[1].Life; got != life0-4 {
		t.Errorf("CR 707.10 Fling seq %d: opponent life %d, want %d (the copy uses the original's sacrificed Grizzly Bears, power 2)", len(e.L.Events), got, life0-4)
	}
	replayCheck(t, e, cfg)
}

// TestCR707_10CopiedCorpseLungeUsesTheOriginalExile: Corpse Lunge exiles
// Grizzly Bears (power 2) from the graveyard as its cost and targets the
// opponent's Colossal Dreadmaw; Twincast's copy keeps the target. The copy
// reads the card exiled for the original (castExiled): the Dreadmaw takes 4
// (today the copy deals 0: 2).
func TestCR707_10CopiedCorpseLungeUsesTheOriginalExile(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{choiceCorpusCard(t, "Corpse Lunge"), choiceCorpusCard(t, "Twincast"), choiceCorpusCard(t, "Grizzly Bears")},
		[]*cards.Card{choiceCorpusCard(t, "Colossal Dreadmaw")})
	lunge := moveByName(t, e, 0, "Corpse Lunge", state.ZHand)
	twin := moveByName(t, e, 0, "Twincast", state.ZHand)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	maw := moveByName(t, e, 1, "Colossal Dreadmaw", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()

	addMana(t, e, 0, "BBB")
	cr707CastCostPaid(t, e, lunge, bears, maw)
	if o := e.G.Obj(bears); o == nil || o.Zone != state.ZExile {
		t.Fatalf("CR 707.10 Corpse Lunge seq %d: Grizzly Bears was not exiled", len(e.L.Events))
	}
	cr707TwincastAndResolve(t, e, twin, lunge, maw)

	if got := e.G.Obj(maw).Damage; got != 4 {
		t.Errorf("CR 707.10 Corpse Lunge seq %d: Colossal Dreadmaw damage %d, want 4 (the copy uses the original's exiled Grizzly Bears, power 2)", len(e.L.Events), got)
	}
	replayCheck(t, e, cfg)
}
