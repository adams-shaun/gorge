package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	twoManaSpellSrc = "Name:Two Spell\nManaCost:1 R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	mountainSrc     = "Name:Alt Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
)

// plannedSpellOffered builds a game whose seat 0 holds the two-mana spell, one
// untapped Mountain, and the single floating unit add describes, and reports
// whether the payment planner lists the spell's cast (the autopay surface: the
// ordinary offer walk prices the floating pool alone, so with one unit
// floating only the planner can offer a {1}{R} spell).
func plannedSpellOffered(t *testing.T, add events.Event) bool {
	t.Helper()
	e, _, spell := newFixtureDeck(t, 30, twoManaSpellSrc, bearSrc)
	toMain1(t, e)
	onBoardGrant(t, e, 0, mountainSrc)
	add.Kind, add.Player, add.Amount = events.ManaAdd, 0, 1
	e.emit(add)
	e.priorityRound()
	pl := &e.G.Players[0]
	if pl.Pool.Total() != 1 || e.G.Obj(spell).Zone != state.ZHand {
		t.Fatalf("precondition: pool %v, spell in %s; want one floating unit and the spell in hand", pl.Pool, e.G.Obj(spell).Zone)
	}
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	e.EnsurePaymentActions()
	return ppHasAction(d, spell)
}

// TestPlannerPaysPastATreasureUnit: CR 601.2g-h lets the player pay the total
// cost from the pool and from untapped sources together, so a floating Treasure
// {C} plus an untapped Mountain pays {1}{R}: the cast is offered. The plain {C}
// unit and the artifact-tagged ones (Treasure, Sol Ring, a Desert) are the same
// to the planner, and the empty-pool control shows the Mountain alone does not
// pay, so the offer comes from the unit.
func TestPlannerPaysPastATreasureUnit(t *testing.T) {
	t.Parallel()
	for _, counter := range []string{"C", "TreasureC", "ArtifactC", "DesertC"} {
		if !plannedSpellOffered(t, events.Event{Counter: counter}) {
			t.Errorf("a {1}{R} spell is not offered with a floating %s unit and an untapped Mountain", counter)
		}
	}
	e, _, spell := newFixtureDeck(t, 30, twoManaSpellSrc, bearSrc)
	toMain1(t, e)
	onBoardGrant(t, e, 0, mountainSrc)
	e.priorityRound()
	d := e.Pending()
	e.EnsurePaymentActions()
	if ppHasAction(d, spell) {
		t.Fatal("control: one Mountain alone offers a {1}{R} spell")
	}
}

// TestPlannerPaysPastARestrictedUnit: a floating restricted {C} (CR 106.6)
// counts toward a cast its restriction admits and not toward one it does not,
// and either way the untapped land pays the rest.
func TestPlannerPaysPastARestrictedUnit(t *testing.T) {
	t.Parallel()
	if !plannedSpellOffered(t, events.Event{Counter: "C", Text: events.ManaRestrictionText("Spell.Creature", 0)}) {
		t.Error("a creature spell is not offered with a floating creature-only {C} and an untapped Mountain")
	}
	if plannedSpellOffered(t, events.Event{Counter: "C", Text: events.ManaRestrictionText("Spell.Instant", 0)}) {
		t.Error("a creature spell is offered with a floating instant-only {C}: the restricted unit may not pay it (CR 106.6)")
	}
}
