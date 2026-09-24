package host

import (
	"testing"
)

// TestTableInfoCarriesRestartConfig pins the two additive wire fields a
// restart control reads to recreate the current game: TableInfo.Mulligans
// (the London allowance, public table config like Format/BotPolicy) and
// SeatInfo.DeckID (the EXACT pool id a seat plays, not the deck's display
// Name the SeatInfo.Deck field carries). Both must be present on the real
// wire shapes a client sees: Tables() for the table, and the match's own
// seat list for the decks.
func TestTableInfoCarriesRestartConfig(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	cfg := fourSeatTable("t1", false)
	cfg.Seats = 2
	cfg.Decks = []string{"a", "b"}
	// A non-default allowance so the assertion can actually distinguish the
	// field from the zero value (the test would pass vacuously if we
	// asserted 0 against the zero TableConfig).
	cfg.Mulligans = 2
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	ti := r.Tables()
	if len(ti) != 1 {
		t.Fatalf("Tables() returned %d rows, want 1", len(ti))
	}
	if got := ti[0].Mulligans; got != 2 {
		t.Fatalf("TableInfo.Mulligans = %d, want 2 (the configured allowance)", got)
	}

	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches %v, %v", ms, err)
	}
	seats := ms[0].Seats
	if len(seats) != 2 {
		t.Fatalf("match seat list has %d seats, want 2", len(seats))
	}
	// DeckID is the pool id from TableConfig.Decks, in seat order. Match 1
	// rotates Decks by one (host/match.go's (i+k)%len), so seat 0 plays
	// decks[1] and seat 1 plays decks[0]. The two seats differ, so a
	// copy-pasted single value cannot pass.
	if seats[0].DeckID != "b" || seats[1].DeckID != "a" {
		t.Fatalf("SeatInfo.DeckID = %q, %q; want \"b\", \"a\"", seats[0].DeckID, seats[1].DeckID)
	}
	for i, s := range seats {
		if s.DeckID == "" {
			t.Fatalf("seat %d: DeckID is empty (field not wired)", i)
		}
	}
}
