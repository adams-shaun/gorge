package rules

// cr-g134: AnimateAll reads RemoveKeywords$ and RemoveAllAbilities$ (the
// shared Animate parser already parsed both; the sweep used to clear them
// behind an unread note). The end-to-end consequence is pinned on the two
// corpus carriers whose whole point is the strip: Spectacular Pileup ("all
// creatures and Vehicles lose indestructible until end of turn, then destroy
// all creatures and Vehicles" — CR 702.12b only bites AFTER the loss) and
// Final Showdown's {1} Spree mode ("all creatures lose all abilities until
// end of turn"). Both fixtures replay: every placement is a logged MoveZone.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// animateAllEngine seeds a 2-seat game whose seat 0 carries the spell and the
// permanent plus Mountains (so the seed-dependent toss winner is pinned to
// seat 0 by seatZeroStart), drives to turn-1 Main 1, bridges the spell into
// the hand and the permanent onto the battlefield through logged MoveZones,
// then re-asks priority.
func animateAllEngine(t *testing.T, seed uint64, spell, perm string) (*Engine, Config, map[string]state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{mustCorpusCard(t, reg, spell), mustCorpusCard(t, reg, perm)},
				mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	ids := map[string]state.ObjID{
		"spell": findByName(e, spell, 0),
		"perm":  findByName(e, perm, 0),
	}
	if ids["spell"] == 0 || ids["perm"] == 0 {
		t.Fatalf("corpus cards not found: spell=%d perm=%d", ids["spell"], ids["perm"])
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: ids["spell"],
		From: e.G.Obj(ids["spell"]).Zone, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: ids["perm"],
		From: e.G.Obj(ids["perm"]).Zone, To: state.ZHand})
	placeOnBattlefield(t, e, ids["perm"])
	e.pending = nil
	e.priorityRound()
	return e, cfg, ids
}

// TestAnimateAllRemoveKeywordsSpectacularPileup: the "lose indestructible,
// then destroy" carrier. The Darksteel Myr must DIE — before cr-g134 the
// sweep dropped RemoveKeywords$, the Myr stayed indestructible through
// DBDestroyAll and the board kept it.
func TestAnimateAllRemoveKeywordsSpectacularPileup(t *testing.T) {
	t.Parallel()
	e, cfg, ids := animateAllEngine(t, 1207, "Spectacular Pileup", "Darksteel Myr")
	if e.G.Obj(ids["perm"]).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Darksteel Myr zone = %s, want battlefield", e.G.Obj(ids["perm"]).Zone)
	}
	if !e.HasKeyword(ids["perm"], "Indestructible") {
		t.Fatal("precondition: Darksteel Myr does not read Indestructible")
	}
	for range 5 { // {3}{W}{W}
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	}
	submitOption(t, e, "", "Cast Spectacular Pileup")
	passUntilStackEmpty(t, e, 30)
	o := e.G.Obj(ids["perm"])
	if o.Zone != state.ZGraveyard {
		t.Fatalf("Darksteel Myr zone after Spectacular Pileup = %s, want graveyard (it must lose Indestructible before the destroy)", o.Zone)
	}
	found := false
	for _, id := range e.G.Zone(state.ZGraveyard, 0) {
		if id == ids["perm"] {
			found = true
		}
	}
	if !found {
		t.Fatalf("Darksteel Myr id %d not in seat 0's graveyard list: %v", ids["perm"], e.G.Zone(state.ZGraveyard, 0))
	}
	replayCheck(t, e, cfg)
}

// TestAnimateAllRemoveAllAbilitiesFinalShowdown: Final Showdown's {1} Spree
// mode ("all creatures lose all abilities until end of turn"). The Myr must
// stop reading Indestructible for the rest of the turn and read it again at
// the next turn (the removal is until end of turn).
func TestAnimateAllRemoveAllAbilitiesFinalShowdown(t *testing.T) {
	t.Parallel()
	e, cfg, ids := animateAllEngine(t, 1310, "Final Showdown", "Darksteel Myr")
	if e.G.Obj(ids["perm"]).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Darksteel Myr zone = %s, want battlefield", e.G.Obj(ids["perm"]).Zone)
	}
	if !e.HasKeyword(ids["perm"], "Indestructible") {
		t.Fatal("precondition: Darksteel Myr does not read Indestructible")
	}
	for range 2 { // printed {W} + the {1} mode
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	}
	submitOption(t, e, "", "Cast Final Showdown")
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast, next decision = %+v, want modes", d)
	}
	submitChoices(t, e, modeOptionContaining(t, d, "lose all abilities"))
	passUntilStackEmpty(t, e, 30)
	if e.G.Obj(ids["spell"]).Zone != state.ZGraveyard {
		t.Fatalf("Final Showdown zone after resolution = %s, want graveyard", e.G.Obj(ids["spell"]).Zone)
	}
	if e.HasKeyword(ids["perm"], "Indestructible") {
		t.Fatal("Darksteel Myr still reads Indestructible after the lose-all-abilities mode resolved")
	}
	// Next turn: the until-end-of-turn removal has expired.
	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	if !e.HasKeyword(ids["perm"], "Indestructible") {
		t.Fatal("Darksteel Myr lost Indestructible into the next turn (the removal must be until end of turn)")
	}
	replayCheck(t, e, cfg)
}
