package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// awakenScenario deals Coastal Discovery into seat 0's hand with a land on
// the battlefield and funds the pool with six blue mana.
func awakenScenario(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Coastal Discovery", "Island")
	id := moveNamedToHand(t, e, "Coastal Discovery")
	land := putNamedOnBattlefield(t, e, "Island")
	addMana(t, e, 0, "UUUUUU")
	return e, cfg, id, land
}

// answerUntilStackEmpty answers every non-priority decision with option 0 and
// passes priority until the stack is empty again.
func answerUntilStackEmpty(t *testing.T, e *Engine) {
	t.Helper()
	for range 40 {
		d := e.Pending()
		if d == nil {
			return
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				return
			}
			passUntilStackEmpty(t, e, 20)
			continue
		}
		submitChoices(t, e, 0)
	}
	t.Fatal("decisions never settled")
}

// CR 702.113a: casting for the awaken cost also puts N +1/+1 counters on a
// target land you control and animates it (0/0 Elemental creature, haste).
// The plain cast does neither.
func TestAwakenAnimatesTargetLand(t *testing.T) {
	t.Parallel()
	t.Run("awakened", func(t *testing.T) {
		e, cfg, id, land := awakenScenario(t)
		offer := altCastOffer(t, e, id, "awakened")
		if offer == nil {
			t.Fatalf("no (awakened) cast offer for Coastal Discovery: %+v", castOptions(t, e))
		}
		if o := e.G.Obj(land); o == nil || o.Zone != state.ZBattlefield || e.IsCreature(land) {
			t.Fatalf("precondition: the Island must be a non-creature land on the battlefield, got %+v", o)
		}
		hand := len(e.G.Zone(state.ZHand, 0))
		pool := e.G.Players[0].Pool.Total()
		submitChoices(t, e, offer.Index)
		answerUntilStackEmpty(t, e)
		if got := pool - e.G.Players[0].Pool.Total(); got != 6 {
			t.Fatalf("awakened cast paid %d mana, want 6 ({5}{U})", got)
		}
		if got := len(e.G.Zone(state.ZHand, 0)); got != hand-1+2 {
			t.Fatalf("hand = %d, want %d (cast one, drew two)", got, hand-1+2)
		}
		o := e.G.Obj(land)
		if got := o.Counter("P1P1"); got != 4 {
			t.Fatalf("awakened land P1P1 = %d, want 4", got)
		}
		d := e.Derived(land)
		if !e.IsCreature(land) || !slices.Contains(d.Types, "Land") || !slices.Contains(d.Types, "Elemental") ||
			d.Power != 4 || d.Toughness != 4 || !slices.Contains(d.Keywords, "Haste") {
			t.Fatalf("awakened land: types %v %d/%d keywords %v, want a 4/4 Elemental creature land with haste",
				d.Types, d.Power, d.Toughness, d.Keywords)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("plain", func(t *testing.T) {
		e, cfg, id, land := awakenScenario(t)
		offer := altCastOffer(t, e, id, "")
		if offer == nil {
			t.Fatal("no plain cast offer for Coastal Discovery")
		}
		hand := len(e.G.Zone(state.ZHand, 0))
		submitChoices(t, e, offer.Index)
		if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
			t.Fatalf("plain cast asked for a target: %+v", d)
		}
		answerUntilStackEmpty(t, e)
		if got := len(e.G.Zone(state.ZHand, 0)); got != hand-1+2 {
			t.Fatalf("hand = %d, want %d (cast one, drew two)", got, hand-1+2)
		}
		if got := e.G.Obj(land).Counter("P1P1"); got != 0 || e.IsCreature(land) {
			t.Fatalf("plain cast touched the land: P1P1 %d, creature %v", got, e.IsCreature(land))
		}
		replayCheck(t, e, cfg)
	})
}

// CR 702.113a with a targeted spell: Clutch of Currents announces its creature
// target and the awakened cast adds the land target; the awaken mode is
// withheld when the caster controls no land to target, the plain cast stays.
func TestAwakenChainsAfterTheSpellsOwnTargetAndNeedsALand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Clutch of Currents", "Island", "Grizzly Bears")
	id := moveNamedToHand(t, e, "Clutch of Currents")
	bear := putNamedOnBattlefield(t, e, "Grizzly Bears")
	addMana(t, e, 0, "UUUU")
	if altCastOffer(t, e, id, "") == nil {
		t.Fatal("plain Clutch of Currents not offered")
	}
	if altCastOffer(t, e, id, "awakened") != nil {
		t.Fatal("awakened cast offered with no land to target")
	}
	land := putNamedOnBattlefield(t, e, "Island")
	addMana(t, e, 0, "UUUU")
	offer := altCastOffer(t, e, id, "awakened")
	if offer == nil {
		t.Fatalf("no (awakened) cast offer once a land is controlled: %+v", castOptions(t, e))
	}
	submitChoices(t, e, offer.Index)
	answerUntilStackEmpty(t, e)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Grizzly Bears = %+v, want it returned to hand", o)
	}
	if got := e.G.Obj(land).Counter("P1P1"); got != 3 || !e.IsCreature(land) {
		t.Fatalf("awakened land: P1P1 %d creature %v, want 3 and a creature (Awaken 3)", got, e.IsCreature(land))
	}
	replayCheck(t, e, cfg)
}
