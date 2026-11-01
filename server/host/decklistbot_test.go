package host

// Operator ruling 2026-10-13: a bot policy that reads every seat's real
// decklist (bots.Info.Env; server/host/botenv.go envData builds the search root
// from m.gc.Decks) never seats at a table with a human, and runs only on
// a bot-only table that explicitly opted in via TableConfig's
// AllowDecklistBots. These tests pin both halves of that backstop on the
// real host table validation, using the registered sb-tactical entry (Env
// without Search, so no search deps are needed) and asserting the
// precondition the whole file leans on: the policy really is a
// decklist reader.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

const decklistPolicy = "sb-tactical"

func TestDecklistBotRefusedAtHumanTable(t *testing.T) {
	e, ok := bots.Lookup(decklistPolicy)
	if !ok {
		t.Fatal("bots registry has no \"sb-tactical\" entry")
	}
	// Precondition: the policy under test really reads decklists, or both
	// arms below would pass vacuously.
	if !e.Env {
		t.Fatalf("%q has Env=false; the test needs a decklist-reading policy", decklistPolicy)
	}
	for _, c := range []struct {
		name    string
		optIn   bool
		wantMsg string
	}{
		{"without the opt-in", false, "it cannot seat at a table with a human"},
		// The opt-in is for bot-only tables: it can never buy a human seat.
		{"with the opt-in", true, "it cannot seat at a table with a human"},
	} {
		r, err := New(testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		cfg := TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 1,
			Spectator: view.Public, Humans: []int{0}, BotPolicy: decklistPolicy,
			AllowDecklistBots: c.optIn}
		err = r.AddTable(cfg)
		if err == nil {
			t.Errorf("%s: AddTable seated a decklist-reading policy at a human table", c.name)
		} else if !strings.Contains(err.Error(), c.wantMsg) {
			t.Errorf("%s: error %v, want one containing %q", c.name, err, c.wantMsg)
		}
		if n := len(r.Tables()); n != 0 {
			t.Errorf("%s: refused table was retained (%d tables)", c.name, n)
		}
		r.Close()
	}
}

func TestDecklistBotNeedsOptIn(t *testing.T) {
	e, ok := bots.Lookup(decklistPolicy)
	if !ok || !e.Env {
		t.Fatalf("precondition: %q is not a registered decklist-reading policy", decklistPolicy)
	}
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// Bot-only (no Humans), but no opt-in: refused, naming the opt-in.
	err = r.AddTable(TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 1,
		Spectator: view.Public, BotPolicy: decklistPolicy})
	if err == nil {
		t.Fatal("AddTable ran a decklist-reading policy on a bot-only table without allow_decklist_bots")
	}
	if !strings.Contains(err.Error(), "needs allow_decklist_bots on a bot-only table") {
		t.Fatalf("error %v, want one naming the opt-in", err)
	}
	if n := len(r.Tables()); n != 0 {
		t.Fatalf("refused table was retained (%d tables)", n)
	}
}

func TestDecklistBotOptInBotOnlyTable(t *testing.T) {
	e, ok := bots.Lookup(decklistPolicy)
	if !ok || !e.Env {
		t.Fatalf("precondition: %q is not a registered decklist-reading policy", decklistPolicy)
	}
	// sb-tactical's factory reads card facts, so the table's seats need the
	// same card dependency the other hosted-policy tests thread.
	opts := testOptions(t)
	opts.BotDeps = sampleBotDeps(t)
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// Precondition: the identical table without the opt-in is refused, so
	// the match below is evidence the opt-in — not a policy change — let it
	// register.
	if err := r.AddTable(TableConfig{ID: "t0", Seats: 2, Decks: []string{"a", "b"}, Seed: 1,
		Spectator: view.Public, BotPolicy: decklistPolicy}); err == nil {
		t.Fatal("the unopted table registered; the test cannot show the opt-in did anything")
	}
	cfg := TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 20261013,
		Spectator: view.Public, BotPolicy: decklistPolicy, AllowDecklistBots: true}
	if err := r.AddTable(cfg); err != nil {
		t.Fatalf("opted-in bot-only table refused: %v", err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Wait("t1")
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches %v, %v; want exactly one played match", ms, err)
	}
	if ms[0].State != protocol.MatchFinished {
		t.Fatalf("match state %q, want finished", ms[0].State)
	}
}

func TestDecklistBotOptInPersists(t *testing.T) {
	e, ok := bots.Lookup(decklistPolicy)
	if !ok || !e.Env {
		t.Fatalf("precondition: %q is not a registered decklist-reading policy", decklistPolicy)
	}
	dir := t.TempDir()
	{
		opts := diskOptions(t, dir)
		opts.BotDeps = sampleBotDeps(t)
		r, err := New(opts)
		if err != nil {
			t.Fatal(err)
		}
		cfg := TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 20261013,
			Spectator: view.Public, BotPolicy: decklistPolicy, AllowDecklistBots: true}
		if err := r.AddTable(cfg); err != nil {
			t.Fatalf("AddTable: %v", err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatalf("Start: %v", err)
		}
		r.Wait("t1")
		r.Close()
	}
	// The restart re-runs validated() over the persisted tables.json. If the
	// opt-in bit did not round-trip, the load skips the table as invalid and
	// Tables() comes back empty.
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	defer r.Close()
	tabs := r.Tables()
	if len(tabs) != 1 || tabs[0].ID != "t1" || tabs[0].Match != 1 {
		t.Fatalf("tables after restart %+v; want t1 retained with its match (an empty list means the opt-in did not persist and validation skipped the table)", tabs)
	}
}
