package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const openAttractionNote = "open an Attraction (no Attraction deck)"

func attractionNotes(e *Engine, text string) int {
	return countEvents(e, func(ev events.Event) bool { return ev.Kind == events.Note && ev.Text == text })
}

// TestAttractionCardsBecomeSupported: registering OpenAttraction clears the
// api:OpenAttraction reason on the audited cards, and the carriers with other
// missing primitives keep those reasons.
func TestAttractionCardsBecomeSupported(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	if !sup["api:OpenAttraction"] {
		t.Fatal("api:OpenAttraction not registered")
	}
	for _, name := range []string{`"Lifetime" Pass Holder`, "Complaints Clerk"} {
		c := lookup(t, reg, name)
		if got := reg.Unsupported(c, sup); len(got) != 0 {
			t.Errorf("%s: unsupported = %v, want none", name, got)
		}
	}
	others := map[string]string{
		"Command Performance":      "api:PutSticker",
		"Monitor Monitor":          "kw:Once each turn, you may pay {1} to reroll one or more dice you rolled.",
		"Myra the Magnificent":     "trig:VisitAttraction",
		"Pick-a-Beeble":            "api:ClaimThePrize",
		"Soul Swindler":            "count:PlayerCountPropertyYou$AttractionsVisitedThisTurn",
		"Squirrel Squatters":       "count:PlayerCountPropertyYou$AttractionsVisitedThisTurn",
		"The Most Dangerous Gamer": "trig:ClaimPrize",
	}
	for name, want := range others {
		c := lookup(t, reg, name)
		got := reg.Unsupported(c, sup)
		found := false
		for _, r := range got {
			if r == want {
				found = true
			}
			if r == "api:OpenAttraction" {
				t.Errorf("%s still reports api:OpenAttraction", name)
			}
		}
		if !found {
			t.Errorf("%s: unsupported = %v, want it to include %q", name, got, want)
		}
	}
}

func hasRolledToVisitTrigger(c *cards.Card) bool {
	for _, f := range c.Faces {
		for _, tr := range f.Triggers {
			if tr.Mode == "RolledDie" && tr.Params["TriggerZones"] == "Graveyard" &&
				tr.Params["RolledToVisitAttractions"] == "True" {
				return true
			}
		}
	}
	return false
}

// TestLifetimePassHolderOracle: enters tapped, its dies trigger opens an
// Attraction (a no-op without a deck), and a d6 6 in the graveyard does not
// return it (no roll is a roll to visit Attractions).
func TestLifetimePassHolderOracle(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card := lookup(t, reg, `"Lifetime" Pass Holder`)
	if !hasRolledToVisitTrigger(card) {
		t.Fatal("precondition: no RolledDie/Graveyard/RolledToVisitAttractions trigger on the compiled face")
	}
	e, _ := flipEngine(t, reg, 1, []*cards.Card{card}, []*cards.Card{})
	id := moveByName(t, e, 0, `"Lifetime" Pass Holder`, state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || !o.Tapped {
		t.Fatalf("precondition: want a tapped battlefield object, got %+v", o)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if n := attractionNotes(e, openAttractionNote); n != 1 {
		t.Fatalf("no-deck Notes after dying = %d, want 1", n)
	}
	if n := attractionNotes(e, "unimplemented API OpenAttraction"); n != 0 {
		t.Fatalf("unimplemented Notes = %d", n)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("card not in graveyard after dying: %+v", o)
	}
	from := len(e.L.Events)
	e.emit(effects.DieRollNote(id, 0, 6, 6, 6))
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("a d6 6 returned the card: %+v", o)
	}
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.MoveZone && ev.Obj == id {
			t.Fatalf("zone change after the roll: %+v", ev)
		}
	}
	if n := attractionNotes(e, openAttractionNote); n != 1 {
		t.Fatalf("no-deck Notes after the roll = %d, want still 1", n)
	}
}

// TestComplaintsClerkOracle: the ETB opens an Attraction (no-op) and a d6 of 1
// makes one Clown Robot token, a 2 none.
func TestComplaintsClerkOracle(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	run := func(natural int32) int {
		e, _ := flipEngine(t, reg, 1, []*cards.Card{lookup(t, reg, "Complaints Clerk")}, []*cards.Card{})
		id := moveByName(t, e, 0, "Complaints Clerk", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: not on the battlefield: %+v", o)
		}
		e.priorityRound()
		passUntilStackEmpty(t, e, 40)
		if n := attractionNotes(e, openAttractionNote); n != 1 {
			t.Fatalf("ETB no-deck Notes = %d, want 1", n)
		}
		if n := attractionNotes(e, "unimplemented API OpenAttraction"); n != 0 {
			t.Fatalf("unimplemented Notes = %d", n)
		}
		if n := tokensNamed(e, 0, "Clown Robot"); n != 0 {
			t.Fatalf("tokens before any roll = %d", n)
		}
		e.emit(effects.DieRollNote(id, 0, 6, natural, natural))
		e.priorityRound()
		passUntilStackEmpty(t, e, 40)
		return tokensNamed(e, 0, "Clown Robot")
	}
	if got := run(1); got != 1 {
		t.Fatalf("d6 = 1: Clown Robot tokens = %d, want 1", got)
	}
	if got := run(2); got != 0 {
		t.Fatalf("d6 = 2: Clown Robot tokens = %d, want 0", got)
	}
}
