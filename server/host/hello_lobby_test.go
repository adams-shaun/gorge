package host

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestHelloOmitsPrivateTablesButSeatSubscriptionWorks pins the stream's first
// frame to the same table list the lobby serves: a private on-demand vs-bot
// table must not be advertised to every unauthenticated stream. It remains
// addressable by id, so a client holding its seat token still reaches it
// through an explicit focus subscription.
func TestHelloOmitsPrivateTablesButSeatSubscriptionWorks(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(TableConfig{ID: "t1", Name: "Table t1", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 7, Spectator: view.Public}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(TableConfig{ID: "g1", Name: "Private game", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 8, Spectator: view.Public, OnDemand: true}); err != nil {
		t.Fatal(err)
	}
	// Precondition: the private table exists and is addressable.
	if got := r.Tables(); len(got) != 2 {
		t.Fatalf("registry has %d tables, want the public t1 and private g1", len(got))
	}

	anon := r.OpenSession()
	hello := decode[protocol.Hello](t, r.Hello(anon))
	lobby := r.LobbyTables()
	if len(hello.Tables) != len(lobby) || len(hello.Tables) != 1 || hello.Tables[0].ID != "t1" {
		t.Fatalf("hello tables %+v, want exactly the lobby list %+v", hello.Tables, lobby)
	}
	// Without a claim, the private table is not even addressable: the
	// rejection is ErrNotFound so a guessed sequential id learns nothing.
	if err := r.Subscribe(anon, "g1", protocol.ModeFocus); err != ErrNotFound {
		t.Fatalf("anonymous focus subscription to private table g1 = %v, want ErrNotFound", err)
	}

	// A seat-scoped client still gets the private table by subscribing to it.
	seated := r.OpenSession()
	seated.BindSeat("g1")
	if err := r.Subscribe(seated, "g1", protocol.ModeFocus); err != nil {
		t.Fatalf("focus subscription to private table g1 with its claim: %v", err)
	}
}
