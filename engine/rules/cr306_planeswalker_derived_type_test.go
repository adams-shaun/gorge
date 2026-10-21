package rules

// CR 306.8 / 306.9 (and their SBA, CR 704.5i) are keyed on the object BEING a
// planeswalker, a layer-4 characteristic, not on its printed type line.
// Luxior, Giada's Gift: "Equipped permanent isn't a planeswalker and is a
// creature in addition to its other types" -- damage to an equipped walker is
// marked damage on a creature and removes no loyalty. Imprisoned in the Moon
// (a Foundations card) makes a walker a colourless land: with no loyalty
// counters it is not put into the graveyard, since it is no planeswalker.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// luxiorWalker builds dsBoardWith's board (seat 1 holds seat1Card) with Ajani,
// Caller of the Pride (loyalty 4) wearing Luxior, and checks the Luxior
// preconditions: a 4/4 creature that is not a planeswalker.
func luxiorWalker(t *testing.T, seat1Card *cards.Card) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := dsBoardWith(t, reg, seat1Card, "Ajani, Caller of the Pride", "Luxior, Giada's Gift")
	ajani, lux := ids["Ajani, Caller of the Pride"], ids["Luxior, Giada's Gift"]
	e.emit(events.Event{Kind: events.Attach, Obj: lux, IDs: []state.ObjID{ajani}})
	e.pending = nil
	e.Advance()
	o := e.G.Obj(ajani)
	if o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 4 || e.G.Obj(lux).AttachedTo != ajani {
		t.Fatalf("precondition: Ajani in %v with %d loyalty, Luxior on %d", o.Zone, o.Counter("LOYALTY"), e.G.Obj(lux).AttachedTo)
	}
	if !e.IsCreature(ajani) || slices.Contains(e.derivedTypesOf(ajani), "Planeswalker") {
		t.Fatalf("precondition: Luxior's Ajani has types %v, want a creature that is not a planeswalker", e.derivedTypesOf(ajani))
	}
	if p, tt := e.Power(ajani), e.Toughness(ajani); p != 4 || tt != 4 {
		t.Fatalf("precondition: Luxior's Ajani is %d/%d, want 4/4 (a +1/+1 per counter)", p, tt)
	}
	return e, cfg, ajani
}

// TestCR306_8LuxiorWalkerBoltIsMarkedDamage: Lightning Bolt at a Luxior'd
// Ajani (4 loyalty, so 4/4) deals 3 damage to a CREATURE that is not a
// planeswalker: 3 marked damage, loyalty stays 4, and the 4/4 survives.
func TestCR306_8LuxiorWalkerBoltIsMarkedDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bolt := mustCorpusCard(t, reg, "Lightning Bolt")
	e, cfg, ajani := luxiorWalker(t, bolt)
	cardToHandSeat(t, e, 1, bolt)
	addMana(t, e, 1, "R")
	passFirst(t, e)
	d := e.Pending()
	if d == nil {
		t.Fatal("seat 1 got no decision after the pass")
	}
	cast := false
	for _, o := range d.Options {
		if o.Kind == "cast" {
			submitChoices(t, e, o.Index)
			targetObject(t, e, ajani)
			cast = true
			break
		}
	}
	if !cast {
		t.Fatalf("seat 1 was not offered Lightning Bolt: %+v", d.Options)
	}
	drainStack(t, e, 20)
	o := e.G.Obj(ajani)
	if o.Zone != state.ZBattlefield || o.Damage != 3 || o.Counter("LOYALTY") != 4 {
		t.Errorf("CR 306.8 seq %d: Luxior's Ajani after Lightning Bolt is in %v with %d marked damage and %d loyalty, want the battlefield, 3 marked, 4 loyalty (it is not a planeswalker)",
			len(e.L.Events), o.Zone, o.Damage, o.Counter("LOYALTY"))
	}
	replayCheck(t, e, cfg)
}

// TestCR306_8LuxiorWalkerUntaggedDamageIsMarked: the same rule on a Damage
// event with no recipient tag -- the shape combat damage emits -- so the
// fold's own planeswalker test is what decides.
func TestCR306_8LuxiorWalkerUntaggedDamageIsMarked(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ajani := luxiorWalker(t, mustCorpusCard(t, reg, "Mountain"))
	e.emit(events.Event{Kind: events.Damage, Obj: ajani, Amount: 2})
	e.pending = nil
	e.Advance()
	o := e.G.Obj(ajani)
	if o.Zone != state.ZBattlefield || o.Damage != 2 || o.Counter("LOYALTY") != 4 {
		t.Errorf("CR 306.8 seq %d: Luxior's Ajani after 2 untagged damage is in %v with %d marked damage and %d loyalty, want the battlefield, 2 marked, 4 loyalty",
			len(e.L.Events), o.Zone, o.Damage, o.Counter("LOYALTY"))
	}
	replayCheck(t, e, cfg)
}

// TestCR306_9MoonedWalkerWithNoLoyaltyStays: Imprisoned in the Moon makes
// Ajani a colourless land. Removing its last loyalty counter does not put it
// into the graveyard: CR 704.5i applies to planeswalkers only.
func TestCR306_9MoonedWalkerWithNoLoyaltyStays(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Ajani, Caller of the Pride"), lookup(t, reg, "Imprisoned in the Moon")}, nil)
	ajani := moveCorpusCard(t, e, "Ajani, Caller of the Pride", 0, state.ZBattlefield)
	moveCorpusCard(t, e, "Imprisoned in the Moon", 0, state.ZHand)
	lossAttach(t, e, "Imprisoned in the Moon", 0, ajani)
	if slices.Contains(e.derivedTypesOf(ajani), "Planeswalker") || !e.IsLand(ajani) {
		t.Fatalf("precondition: the Mooned Ajani has types %v, want a land that is not a planeswalker", e.derivedTypesOf(ajani))
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: ajani, Counter: "LOYALTY", Amount: -e.G.Obj(ajani).Counter("LOYALTY")})
	e.pending = nil
	e.Advance()
	if o := e.G.Obj(ajani); o.Zone != state.ZBattlefield {
		t.Errorf("CR 306.9/704.5i seq %d: the Mooned Ajani with 0 loyalty is in %v, want the battlefield (it is not a planeswalker)", len(e.L.Events), o.Zone)
	}
	replayCheck(t, e, cfg)
}

// TestCR306_8PlaneswalkerGuards pins what stays right on a plain walker:
// Lightning Bolt removes 3 loyalty and marks nothing (CR 306.8), and a walker
// at 0 loyalty goes to the graveyard (CR 306.9/704.5i).
func TestCR306_8PlaneswalkerGuards(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("bolt removes loyalty", func(t *testing.T) {
		e, cfg, ids := dsBoardWith(t, reg, mustCorpusCard(t, reg, "Mountain"), "Ajani, Caller of the Pride")
		ajani := ids["Ajani, Caller of the Pride"]
		e.emit(events.Event{Kind: events.Damage, Obj: ajani, Amount: 3})
		e.pending = nil
		e.Advance()
		if o := e.G.Obj(ajani); o.Zone != state.ZBattlefield || o.Damage != 0 || o.Counter("LOYALTY") != 1 {
			t.Errorf("CR 306.8 seq %d: Ajani after 3 damage is in %v with %d marked and %d loyalty, want the battlefield, 0 marked, 1 loyalty",
				len(e.L.Events), o.Zone, o.Damage, o.Counter("LOYALTY"))
		}
		replayCheck(t, e, cfg)
	})
	t.Run("zero loyalty dies", func(t *testing.T) {
		e, cfg, ids := dsBoardWith(t, reg, mustCorpusCard(t, reg, "Mountain"), "Ajani, Caller of the Pride")
		ajani := ids["Ajani, Caller of the Pride"]
		e.emit(events.Event{Kind: events.CounterChange, Obj: ajani, Counter: "LOYALTY", Amount: -4})
		e.pending = nil
		e.Advance()
		if o := e.G.Obj(ajani); o.Zone != state.ZGraveyard {
			t.Errorf("CR 704.5i seq %d: Ajani at 0 loyalty is in %v, want the graveyard", len(e.L.Events), o.Zone)
		}
		replayCheck(t, e, cfg)
	})
}
