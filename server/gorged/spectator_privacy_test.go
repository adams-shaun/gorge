package gorged

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDefaultSpectatorPrivacyOnHumanTable starts a server from the real flag
// defaults with a human seat, then reads the spectator view with no token: it
// must be the public projection, with no player's hand contents. Before the
// default moved to public this returned both hands.
func TestDefaultSpectatorPrivacyOnHumanTable(t *testing.T) {
	fs, c := serveFlags()
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if c.spectator != "public" {
		t.Fatalf("-spectator default = %q, want public", c.spectator)
	}
	c.cards = testutil.CorpusDirOrSkip(t)
	c.decks = "../../testkit/testutil/decks"
	c.tables, c.seats = 1, 2
	c.pace, c.cooldown = 0, 0
	c.dir = t.TempDir()
	c.feedback = t.TempDir()
	c.humansRaw = "0,1"
	c.perpetual = false
	c.prewarm = false
	url, cancel, _ := startServe(t, *c)
	defer cancel()
	waitTables(t, url, 1)

	v := spectatorView(t, url)
	if v.Visibility != "public" {
		t.Fatalf("spectator visibility = %q, want public", v.Visibility)
	}
	if len(v.Players) != 2 {
		t.Fatalf("spectator view has %d players, want 2", len(v.Players))
	}
	for _, p := range v.Players {
		if p.Hand != nil {
			t.Fatalf("seat %d hand leaked to a no-token spectator: %d cards", p.ID, len(p.Hand))
		}
	}
}

// spectatorView polls the live match's no-token view until it reports both
// seats, so the request never races the table starting.
func spectatorView(t *testing.T, url string) view.View {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(url + "/api/tables/t1/matches/1/view")
		if err == nil {
			var v view.View
			err = json.NewDecoder(resp.Body).Decode(&v)
			resp.Body.Close()
			if err == nil && len(v.Players) == 2 {
				return v
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no two-seat spectator view served (err %v)", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
