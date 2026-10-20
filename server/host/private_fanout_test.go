package host

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestAnonymousOverviewDoesNotSeePrivateTable pins the wildcard path shut:
// the web opens a "*" overview on every page, so an unauthenticated session
// must not receive match_start (or a widget) for a private on-demand table.
// Only a session whose seat claim names the table may see it.
func TestAnonymousOverviewDoesNotSeePrivateTable(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(TableConfig{ID: "g1", Name: "Private game", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 8, Spectator: view.Public, OnDemand: true}); err != nil {
		t.Fatal(err)
	}
	// Precondition: the private table exists and is startable, so its absence
	// below is the visibility filter and not a missing fixture.
	if got := r.Tables(); len(got) != 1 || got[0].ID != "g1" {
		t.Fatalf("registry tables %+v, want the private g1", got)
	}

	anon := r.OpenSession()
	if err := r.Subscribe(anon, TableAll, protocol.ModeOverview); err != nil {
		t.Fatalf("overview subscription: %v", err)
	}
	if err := r.Start("g1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("g1")

	for _, f := range drainNow(anon) {
		if f.Table == "g1" {
			t.Fatalf("anonymous overview received %s for private table g1", f.T)
		}
	}
	for _, f := range anon.TakeWidgets() {
		if f.Table == "g1" {
			t.Fatal("anonymous overview received a widget for private table g1")
		}
	}
}
