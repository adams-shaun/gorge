package host

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// eventTable is a public event table: on-demand (seatpolicy.go requires it)
// but not a private play-vs-bot game. The event hub links to it without a
// seat token, so it must stay watchable by an anonymous spectator.
func eventTable(id TableID) TableConfig {
	return TableConfig{ID: id, Name: "Event " + string(id), Seats: 2,
		Decks: []string{"a", "b"}, Seed: 9, Spectator: view.Public,
		OnDemand: true, Event: "e1"}
}

// TestAnonymousFocusSubscribeToPublicEventTable pins the privacy fence to
// play-vs-bot tables: an event table is on-demand but public, so an anonymous
// focus subscription (the hub's /t/<table> Watch link) must succeed while the
// private g1 still answers ErrNotFound.
func TestAnonymousFocusSubscribeToPublicEventTable(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(eventTable("e1t")); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(TableConfig{ID: "g1", Name: "Private game", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 8, Spectator: view.Public, OnDemand: true}); err != nil {
		t.Fatal(err)
	}
	// Precondition: both tables exist, so the results below are the claim
	// fence and not a missing fixture.
	if got := r.Tables(); len(got) != 2 {
		t.Fatalf("registry has %d tables, want event e1t and private g1", len(got))
	}

	anon := r.OpenSession()
	if err := r.Subscribe(anon, "e1t", protocol.ModeFocus); err != nil {
		t.Fatalf("anonymous focus subscription to public event table e1t: %v", err)
	}
	if err := r.Subscribe(anon, "g1", protocol.ModeFocus); err != ErrNotFound {
		t.Fatalf("anonymous focus subscription to private g1 = %v, want ErrNotFound", err)
	}
}

// TestAnonymousOverviewSeesPublicEventTable pins the fan-out half: the
// wildcard overview must still carry a public event table's frames. Only the
// play-vs-bot table is skipped.
func TestAnonymousOverviewSeesPublicEventTable(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(eventTable("e1t")); err != nil {
		t.Fatal(err)
	}

	anon := r.OpenSession()
	if err := r.Subscribe(anon, TableAll, protocol.ModeOverview); err != nil {
		t.Fatalf("overview subscription: %v", err)
	}
	if err := r.Start("e1t"); err != nil {
		t.Fatal(err)
	}
	r.Wait("e1t")

	for _, f := range drainNow(anon) {
		if f.Table == "e1t" {
			return
		}
	}
	t.Fatal("anonymous overview received no frames for public event table e1t")
}
