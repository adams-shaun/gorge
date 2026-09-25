package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func charmScopeFixture(t *testing.T, cardName string) (*Engine, Config, state.ObjID, *cards.SA) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card := mustCorpusCard(t, reg, cardName)
	cfg := seatZeroStart(Config{Seed: 78341, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{append([]*cards.Card{card}, mountainDeck(t, 39)...), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	source := placeFromDeck(t, e, 0, cardName)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	o := e.G.Obj(source)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("fixture precondition: %q source=%+v, want controlled battlefield permanent", cardName, o)
	}
	sa := cards.ResolveSVar(o.Face().SVars, "TrigCharm")
	if sa == nil {
		t.Fatalf("%s TrigCharm did not compile", cardName)
	}
	return e, cfg, source, sa
}

func assertCharmModes(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("eligible modes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("eligible modes = %v, want %v", got, want)
		}
	}
}

func recordCharmMode(t *testing.T, e *Engine, source state.ObjID, sa *cards.SA, name string) {
	t.Helper()
	effects.RecordCharmChoices(e, source, sa, []string{name})
	o := e.G.Obj(source)
	if len(o.ModeChoices) == 0 || o.ModeChoices[len(o.ModeChoices)-1].Mode != name {
		t.Fatalf("pick %q not recorded on source: %+v", name, o.ModeChoices)
	}
}

func beginCharmCombat(e *Engine) {
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
}

func TestCharmChoiceRestrictionYourLastCombatByElspethsCommand(t *testing.T) {
	e, cfg, source, sa := charmScopeFixture(t, "By Elspeth's Command")
	if got := sa.Params["ChoiceRestriction"]; got != state.ModeScopeYourLastCombat {
		t.Fatalf("compiled ChoiceRestriction$ = %q", got)
	}
	all := []string{"PumpField", "PumpHand", "Token"}
	beginCharmCombat(e)
	assertCharmModes(t, effects.CharmEligibleModes(e, source, sa, all), all)
	recordCharmMode(t, e, source, sa, "PumpField")
	first := e.G.Obj(source).ModeChoices[len(e.G.Obj(source).ModeChoices)-1]
	if first.Turn != e.G.Turn || first.Combat != e.G.CombatsThisTurn {
		t.Fatalf("pick combat stamp = (%d,%d), clock=(%d,%d)", first.Turn, first.Combat, e.G.Turn, e.G.CombatsThisTurn)
	}

	// TurnChange preserves the prior combat identity and pick; the following
	// BeginCombat rotates it into the forbidden set for this combat.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 3})
	beginCharmCombat(e)
	second := effects.CharmEligibleModes(e, source, sa, all)
	assertCharmModes(t, second, []string{"PumpHand", "Token"})
	recordCharmMode(t, e, source, sa, "PumpHand")

	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 4})
	beginCharmCombat(e)
	third := effects.CharmEligibleModes(e, source, sa, all)
	assertCharmModes(t, third, []string{"PumpField", "Token"})
	if !containsMode(third, "PumpField") || containsMode(third, "PumpHand") {
		t.Fatalf("third combat did not age the first pick and retain the second: %v", third)
	}
	replayCheck(t, e, cfg)
}

func TestCharmChoiceRestrictionYourLastCombatPicklessCombatReleasesPick(t *testing.T) {
	e, _, source, sa := charmScopeFixture(t, "By Elspeth's Command")
	all := []string{"PumpField", "PumpHand", "Token"}
	beginCharmCombat(e)
	recordCharmMode(t, e, source, sa, "PumpField")
	beginCharmCombat(e) // No pick: the trigger was countered or suppressed.
	assertCharmModes(t, effects.CharmEligibleModes(e, source, sa, all), []string{"PumpHand", "Token"})
	beginCharmCombat(e)
	got := effects.CharmEligibleModes(e, source, sa, all)
	assertCharmModes(t, got, all)
	if len(e.G.Obj(source).ModeChoices) != 0 {
		t.Fatalf("old pick survived a pickless intervening combat: %+v", e.G.Obj(source).ModeChoices)
	}
}

func TestCharmChoiceRestrictionYourLastCombatYotianCourier(t *testing.T) {
	e, _, source, sa := charmScopeFixture(t, "Yotian Courier")
	if sa.Params["ChoiceRestriction"] != state.ModeScopeYourLastCombat {
		t.Fatalf("compiled ChoiceRestriction$ = %q", sa.Params["ChoiceRestriction"])
	}
	all := []string{"Powerstone", "Seek"}
	beginCharmCombat(e)
	recordCharmMode(t, e, source, sa, "Powerstone")
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 3})
	beginCharmCombat(e)
	got := effects.CharmEligibleModes(e, source, sa, all)
	assertCharmModes(t, got, []string{"Seek"})
}

func TestCharmChoiceRestrictionThisGamePersists(t *testing.T) {
	e, _, source, sa := charmScopeFixture(t, "Silent Hallcreeper")
	if sa.Params["ChoiceRestriction"] != state.ModeScopeThisGame {
		t.Fatalf("compiled ChoiceRestriction$ = %q", sa.Params["ChoiceRestriction"])
	}
	all := []string{"DBPutCounter", "DBDraw", "DBClone"}
	assertCharmModes(t, effects.CharmEligibleModes(e, source, sa, all), all)
	recordCharmMode(t, e, source, sa, "DBDraw")
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: 3})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 4})
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: 5})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 6})
	got := effects.CharmEligibleModes(e, source, sa, all)
	assertCharmModes(t, got, []string{"DBPutCounter", "DBClone"})
}

func TestCharmChoiceRestrictionUnknownScopeNotesAndFailsOpen(t *testing.T) {
	e, _, source, _ := charmScopeFixture(t, "By Elspeth's Command")
	sa := &cards.SA{Params: map[string]string{"ChoiceRestriction": "FutureScope"}}
	modes := []string{"A", "B"}
	before := len(e.L.Events)
	got := effects.CharmEligibleModes(e, source, sa, modes)
	assertCharmModes(t, got, modes)
	if len(e.L.Events) != before+1 || e.L.Events[len(e.L.Events)-1].Kind != events.Note || e.L.Events[len(e.L.Events)-1].Text != "unmodelled ChoiceRestriction$ scope: FutureScope" {
		t.Fatalf("unknown scope did not emit its diagnostic Note: %+v", e.L.Events[before:])
	}
}

func containsMode(modes []string, name string) bool {
	for _, mode := range modes {
		if mode == name {
			return true
		}
	}
	return false
}
