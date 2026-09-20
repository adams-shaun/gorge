package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The FILTERED Count$CastTotalManaSpent <Type> form (task castfilter1),
// pinned end to end on real corpus cards.
//
// Snow is the tractable family: the pool has always carried a PARALLEL snow
// tally (state.Player.Snow, CR 107.4h), so the payment's snow-unit delta is
// real per-unit provenance, captured at payCast and folded into
// Object.ManaSnowSpent. The six Snow carriers (Berg Strider, Blood on the
// Snow, Blessing of Frost, Tundra Fumarole, Graven Lore, Search for Glory)
// all read `Count$CastTotalManaSpent Snow`; before this fix the head returned
// the UNFILTERED total (e.g. 5), and after it returns the snow-sourced count
// (2). Producer-type provenance for Treasure/Cave/Desert does not exist, so
// those fail closed to 0 -- pinned separately below on the ticket's own card.

// TestCastTotalManaSpentSnowCountsOnlySnowEndToEnd casts Tundra Fumarole
// (1 R R, "Add {C} for each {S} spent to cast this spell") from a pool of
// one snow red and two plain red, and asserts the real derived effect: the
// resolution adds exactly the number of COLOURLESS mana equal to the SNOW
// spend (1), not the total spend (3).
func TestCastTotalManaSpentSnowCountsOnlySnowEndToEnd(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Tundra Fumarole"))
	// A target for the spell's 4 damage (the resolution's mana leg runs after).
	target := e.G.AddObject(card(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	target.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{target.ID})
	spell := e.G.Zone(state.ZHand, 0)[0]

	// One SNOW red unit plus two PLAIN red units: the cost {1}{R}{R} is paid
	// entirely from red, but only ONE unit is snow. A correct filtered read is
	// 1; the old unfiltered read was 3.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "SR", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})

	castMode(t, e, spell, "")
	// The spell targets a creature/planeswalker (CR 601.2c): answer the ask; it
	// then pays and the pay-time capture lands on the stack object.
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		pick := -1
		for i, o := range d.Options {
			if o.Obj == target.ID {
				pick = i
			}
		}
		if pick < 0 {
			t.Fatalf("target decision did not offer the bear: %+v", d)
		}
		submitChoices(t, e, pick)
	}
	// The pay-time capture is live on the stack object before resolution.
	if got := e.G.Obj(spell).ManaSpent; got != 3 {
		t.Fatalf("ManaSpent = %d, want 3 (three red units paid {1}{R}{R})", got)
	}
	if got := e.G.Obj(spell).ManaSnowSpent; got != 1 {
		t.Fatalf("ManaSnowSpent = %d, want 1 (one snow unit of three spent)", got)
	}
	// Resolution adds {C} for each snow unit, not each mana spent.
	before := e.G.Players[0].Pool[state.MC]
	e.resolveTop()
	after := e.G.Players[0].Pool[state.MC]
	if got := after - before; got != 1 {
		t.Fatalf("Tundra Fumarole added %d colourless mana, want 1 (one snow unit of three spent)", got)
	}
	if got := target.Damage; got != 4 {
		t.Fatalf("bear took %d damage, want 4", got)
	}
}

// TestCataclysmicProspectingCastTotalManaSpentFailsClosed pins the ticket's
// named carrier. Cataclysmic Prospecting creates a tapped Treasure for each
// mana from a DESERT spent to cast it (SVar:Y:Count$CastTotalManaSpent
// Desert). Desert has no per-unit producer-type provenance, so the head fails
// closed to 0 and the card creates NO Treasures -- the documented direction.
// Before the fix it returned the UNFILTERED total and created too many; this
// test FAILS on unmodified main (3 Treasures for the X=1, three-mana cast)
// because the fail-closed 0 is the new, correct-direction behaviour. The
// assertion is deliberately "0", never a guessed number. Uses the corpus
// token registry so the real TokenScript$ (c_a_treasure_sac) resolves.
func TestCataclysmicProspectingCastTotalManaSpentFailsClosed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, spell := excessFixtureWithTokens(t, 7, corpusCardText(t, "c/cataclysmic_prospecting.txt"), reg.Tokens)
	// Cast for X=1: {X}{R}{R} = 3 mana from three red units (none a Desert --
	// the engine cannot yet tell a Desert from any other land).
	addMana(t, e, 0, "RRR")
	submitChoices(t, e, castOptionFor(t, e, spell).Index)
	// The {X} ask: announce X = 1.
	if d := e.Pending(); d != nil && d.Kind == "choose" {
		submitChoices(t, e, 1)
	}
	// Drain to the resolution.
	for i := 0; i < 40 && len(e.G.Stack) > 0 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack %d)", len(e.G.Stack))
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("non-pass decision while draining: %+v", d)
		}
		submitChoices(t, e, idx)
	}
	if got := tokensNamed(e, 0, "Treasure"); got != 0 {
		t.Fatalf("Cataclysmic Prospecting created %d Treasures, want 0 (no Desert provenance: fail closed, not the unfiltered total)", got)
	}
}
