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
	"github.com/adams-shaun/gorge/engine/effects"
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

// TestCR306_8LuxiorWalkerHealClearsMarkedDamage: heal emits a negative
// untagged Damage event. A Luxior'd walker's damage is ordinary marked damage,
// so the negative event must clear it (the positive-only loyalty exchange must
// not swallow it).
func TestCR306_8LuxiorWalkerHealClearsMarkedDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ajani := luxiorWalker(t, mustCorpusCard(t, reg, "Mountain"))
	e.emit(events.Event{Kind: events.Damage, Obj: ajani, Amount: 2})
	e.pending = nil
	e.Advance()
	if o := e.G.Obj(ajani); o.Damage != 2 || o.Counter("LOYALTY") != 4 {
		t.Fatalf("precondition: Luxior's Ajani has %d marked damage and %d loyalty, want 2 and 4", o.Damage, o.Counter("LOYALTY"))
	}
	effects.HealMarkedDamage(e, ajani)
	e.pending = nil
	e.Advance()
	if o := e.G.Obj(ajani); o.Zone != state.ZBattlefield || o.Damage != 0 || o.Counter("LOYALTY") != 4 {
		t.Errorf("CR 306.8/701.19b seq %d: Luxior's Ajani after a heal is in %v with %d marked damage and %d loyalty, want the battlefield, 0 marked, 4 loyalty",
			len(e.L.Events), o.Zone, o.Damage, o.Counter("LOYALTY"))
	}
	replayCheck(t, e, cfg)
}

// TestCR306_8LuxiorWalkerRegenerationClearsMarkedDamage: regeneration consumes
// its shield and removes marked damage with the same negative untagged Damage
// event. A Luxior'd walker must clear the damage and stay.
func TestCR306_8LuxiorWalkerRegenerationClearsMarkedDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ajani := luxiorWalker(t, mustCorpusCard(t, reg, "Mountain"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: ajani, Counter: "Shield", Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: ajani, Amount: 4})
	e.pending = nil
	e.Advance()
	o := e.G.Obj(ajani)
	if o.Zone != state.ZBattlefield || o.Counter("Shield") != 0 || o.Damage != 0 || o.Counter("LOYALTY") != 4 {
		t.Errorf("CR 306.8/701.19a seq %d: regenerated Luxior's Ajani is in %v with %d shields, %d marked damage and %d loyalty, want the battlefield, 0 shields, 0 marked, 4 loyalty",
			len(e.L.Events), o.Zone, o.Counter("Shield"), o.Damage, o.Counter("LOYALTY"))
	}
	replayCheck(t, e, cfg)
}

// TestCR306_8LuxiorWalkerExcessDamage: the CR 120.10 tally mirrors the fold,
// so a Luxior'd walker is measured by toughness, not loyalty. A +1/+1 counter
// decouples the two (loyalty 4, toughness 5): 5 damage is exactly lethal and
// is NOT excess, whereas the loyalty threshold of 4 would call it excess.
func TestCR306_8LuxiorWalkerExcessDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ajani := luxiorWalker(t, mustCorpusCard(t, reg, "Mountain"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: ajani, Counter: "+1/+1", Amount: 1})
	if p, tt := e.Power(ajani), e.Toughness(ajani); p != 5 || tt != 5 {
		t.Fatalf("precondition: Luxior's Ajani with a +1/+1 counter is %d/%d, want 5/5", p, tt)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: ajani, Amount: 5})
	o := e.G.Obj(ajani)
	if o.Damage != 5 || o.Counter("LOYALTY") != 4 || o.WasDealtExcessDamageThisTurn {
		t.Errorf("CR 120.10 seq %d: Luxior's Ajani after 5 damage has %d marked, %d loyalty, excess %v; want 5 marked, 4 loyalty, excess false (exactly lethal for its 5 toughness, not its 4 loyalty)",
			len(e.L.Events), o.Damage, o.Counter("LOYALTY"), o.WasDealtExcessDamageThisTurn)
	}
	replayCheck(t, e, cfg)
}

// TestCR306_9MoonedWalkerCleanupClearsMarkedDamage: a Mooned walker takes
// ordinary marked damage, and cleanup's negative Damage event must clear it.
func TestCR306_9MoonedWalkerCleanupClearsMarkedDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Ajani, Caller of the Pride"), lookup(t, reg, "Imprisoned in the Moon")}, nil)
	ajani := moveCorpusCard(t, e, "Ajani, Caller of the Pride", 0, state.ZBattlefield)
	moveCorpusCard(t, e, "Imprisoned in the Moon", 0, state.ZHand)
	lossAttach(t, e, "Imprisoned in the Moon", 0, ajani)
	e.emit(events.Event{Kind: events.Damage, Obj: ajani, Amount: 2})
	e.pending = nil
	e.Advance()
	if o := e.G.Obj(ajani); o.Damage != 2 || o.Counter("LOYALTY") != 4 {
		t.Fatalf("precondition: the Mooned Ajani has %d marked damage and %d loyalty, want 2 and 4", o.Damage, o.Counter("LOYALTY"))
	}
	e.cleanupBody()
	if o := e.G.Obj(ajani); o.Zone != state.ZBattlefield || o.Damage != 0 {
		t.Errorf("CR 306.9/514.2 seq %d: the Mooned Ajani after cleanup is in %v with %d marked damage, want the battlefield, 0",
			len(e.L.Events), o.Zone, o.Damage)
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
	t.Run("luxior zero loyalty dies as 0/0", func(t *testing.T) {
		e, cfg, ajani := luxiorWalker(t, mustCorpusCard(t, reg, "Mountain"))
		e.emit(events.Event{Kind: events.CounterChange, Obj: ajani, Counter: "LOYALTY", Amount: -4})
		e.pending = nil
		e.Advance()
		// The planeswalker-zero-loyalty skip does not apply (it is not a
		// planeswalker now), but Luxior's +1/+1 per counter leaves a 0/0
		// creature, which CR 704.5f puts into the graveyard.
		if o := e.G.Obj(ajani); o.Zone != state.ZGraveyard {
			t.Errorf("CR 704.5f seq %d: the 0-loyalty Luxior'd Ajani is in %v, want the graveyard (it is a 0/0 creature)", len(e.L.Events), o.Zone)
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
