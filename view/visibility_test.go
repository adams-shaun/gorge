// This file is package view_test, not view, unlike view_test.go: it needs
// seat.Bot to drive a real game (playSome, below), and seat imports view
// (bot.go takes a view.View) — an internal (package view) test file that
// also imported seat would be a genuine import cycle for Go's test tooling
// (view[.test] -> seat -> view), not merely a style choice. Every symbol
// these tests touch is already exported, so nothing here needs the
// package-private access an internal test file would give up.
package view_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// playSome runs a real 4-seat game for n decisions and returns the engine,
// so visibility is tested against real hands, libraries and a real log.
func playSome(t *testing.T, seed uint64, n int) *rules.Engine {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()
	b := seat.NewBot(seed)
	for i := 0; i < n && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		in, err := b.Decide(context.Background(), view.Project(e.G, e, d.Player, d), *d)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", i, err)
		}
	}
	return e
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVisibilityStringsRoundTrip(t *testing.T) {
	for _, v := range []view.Visibility{view.Seat, view.Public, view.Omniscient} {
		got, err := view.ParseVisibility(v.String())
		if err != nil || got != v {
			t.Errorf("ParseVisibility(%q) = %v, %v", v.String(), got, err)
		}
	}
	if _, err := view.ParseVisibility("godmode"); err == nil {
		t.Error("ParseVisibility accepted an unknown mode")
	}
	if view.Visibility(9).String() != "unknown" {
		t.Error("out-of-range Visibility does not print unknown")
	}
}

func TestProjectForPublicShowsNoHandNoPoolNoDecision(t *testing.T) {
	e := playSome(t, 5, 60)
	v := view.ProjectFor(e.G, e, view.NoSeat, view.Public, e.Pending())
	if v.Visibility != "public" || v.Viewer != view.NoSeat {
		t.Fatalf("header %+v", v)
	}
	for _, p := range v.Players {
		if p.Hand != nil || p.Pool != nil {
			t.Fatalf("public view exposes seat %d's hand or pool", p.ID)
		}
		if p.HandSize != len(e.G.Zone(state.ZHand, p.ID)) {
			t.Fatalf("seat %d hand size %d, want %d", p.ID, p.HandSize, len(e.G.Zone(state.ZHand, p.ID)))
		}
	}
	if v.Decision != nil {
		t.Fatal("public view carries a decision")
	}
}

func TestProjectForOmniscientShowsEveryHandAndPoolButNoLibraryOrder(t *testing.T) {
	e := playSome(t, 5, 60)
	v := view.ProjectFor(e.G, e, view.NoSeat, view.Omniscient, e.Pending())
	if v.Visibility != "omniscient" {
		t.Fatalf("visibility %q", v.Visibility)
	}
	for _, p := range v.Players {
		if p.Hand == nil || p.Pool == nil {
			t.Fatalf("omniscient view hides seat %d's hand or pool", p.ID)
		}
		if len(p.Hand) != len(e.G.Zone(state.ZHand, p.ID)) {
			t.Fatalf("seat %d: %d hand cards projected, %d in hand", p.ID, len(p.Hand), len(e.G.Zone(state.ZHand, p.ID)))
		}
		for _, cv := range p.Hand {
			if cv.Name == "" {
				t.Fatalf("seat %d has an unnamed hand card %+v", p.ID, cv)
			}
		}
		if p.LibrarySize != len(e.G.Zone(state.ZLibrary, p.ID)) {
			t.Fatalf("seat %d library size wrong", p.ID)
		}
	}
	// The View type has no library list at all; this pins that no field
	// was added that could carry one.
	if v.Decision != nil {
		t.Fatal("omniscient spectator carries a seat's decision")
	}
}

func TestProjectForSeatMatchesProject(t *testing.T) {
	e := playSome(t, 8, 40)
	d := e.Pending()
	for p := state.PlayerID(0); p < 4; p++ {
		a := view.Project(e.G, e, p, d)
		b := view.ProjectFor(e.G, e, p, view.Seat, d)
		a.Visibility, b.Visibility = "", ""
		if got, want := mustJSON(t, a), mustJSON(t, b); got != want {
			t.Fatalf("seat %d: ProjectFor(Seat) differs from Project", p)
		}
	}
	if view.Project(e.G, e, 0, d).Visibility != "seat" {
		t.Fatal("Project does not label itself seat")
	}
}

func TestRedactEventsForOmniscientHidesOnlyLibraryOrder(t *testing.T) {
	e := playSome(t, 5, 200)
	out := view.RedactEventsFor(e.G, e.L.Events, view.NoSeat, view.Omniscient)
	if len(out) != len(e.L.Events) {
		t.Fatal("omniscient redaction dropped events")
	}
	shuffles, notes, secretDraws := 0, 0, 0
	for i, ev := range out {
		orig := e.L.Events[i]
		switch {
		case orig.Kind == events.Shuffle:
			shuffles++
			if len(ev.IDs) != 0 {
				t.Fatalf("event %d: omniscient view sees library order", i)
			}
		case orig.Secret && orig.Kind == events.Note:
			notes++
			if ev.Text != "" || len(ev.IDs) != 0 {
				t.Fatalf("event %d: omniscient view sees a private library peek", i)
			}
		case orig.Secret:
			secretDraws++
			if ev.Obj != orig.Obj {
				t.Fatalf("event %d: a hidden-zone move was stripped in omniscient mode", i)
			}
		default:
			if string(ev.Append(nil)) != string(orig.Append(nil)) {
				t.Fatalf("event %d: a public event was altered in omniscient mode", i)
			}
		}
	}
	if shuffles == 0 || secretDraws == 0 {
		t.Fatalf("fixture exercised %d shuffles and %d secret draws; need both", shuffles, secretDraws)
	}
}

func TestRedactEventsForPublicMatchesRedactEventsForASpectator(t *testing.T) {
	e := playSome(t, 5, 200)
	a := view.RedactEvents(e.G, e.L.Events, view.NoSeat)
	b := view.RedactEventsFor(e.G, e.L.Events, view.NoSeat, view.Public)
	if len(a) != len(b) {
		t.Fatal("lengths differ")
	}
	for i := range a {
		if string(a[i].Append(nil)) != string(b[i].Append(nil)) {
			t.Fatalf("event %d differs between RedactEvents and RedactEventsFor(Public)", i)
		}
	}
}

func TestRedactEventsForNeverMutatesItsInput(t *testing.T) {
	e := playSome(t, 5, 100)
	before := make([]string, len(e.L.Events))
	for i, ev := range e.L.Events {
		before[i] = string(ev.Append(nil))
	}
	for _, vis := range []view.Visibility{view.Seat, view.Public, view.Omniscient} {
		out := view.RedactEventsFor(e.G, e.L.Events, view.NoSeat, vis)
		for i := range out {
			if len(out[i].IDs) > 0 {
				out[i].IDs[0] = 4242
			}
		}
	}
	for i, ev := range e.L.Events {
		if string(ev.Append(nil)) != before[i] {
			t.Fatalf("event %d in the engine's log was mutated through a redacted copy", i)
		}
	}
}
