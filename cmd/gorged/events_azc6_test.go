package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc6"
	"github.com/adams-shaun/gorge/internal/draftpersona"
	"github.com/adams-shaun/gorge/internal/draftpersona/personatest"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
)

// TestEventAZC6SeatsDraftWithTheEventDrafter drives a draft event through
// serve() with -event-drafter net:<synthetic net> and two az-c6 bot seats
// (closed-list, enabled over a synthetic az net and belief): az-c6 is an
// offered event policy, every bot seat defaults to the trained persona, the
// draft and build finish, and round 1's series -- az-c6 seats included --
// are played to the end on host tables.
func TestEventAZC6SeatsDraftWithTheEventDrafter(t *testing.T) {
	enableClosed(t, azc6.Policy)
	netDir := t.TempDir()
	w, err := personatest.Write(netDir, "FDN", []string{"Llanowar Elves", "Shivan Dragon", "Serra Angel"})
	if err != nil {
		t.Fatal(err)
	}
	base := startEventsServer(t, config{cards: testutil.CorpusDirOrSkip(t), decks: "../../internal/testutil/decks",
		tables: 1, seats: 2, pace: 0, cooldown: 0, dir: t.TempDir(), spectator: "omniscient", seed: 1,
		events: true, autoMana: true, botAutoPayMana: true, eventDrafter: "net:" + w,
		botPolicies: []string{bots.Default, "sb-tactical", azc6.Policy}})
	req := protocol.CreateEventRequest{Set: "FDN", Seats: []protocol.CreateEventSeat{
		{Kind: "bot", Policy: azc6.Policy}, {Kind: "bot", Policy: azc6.Policy},
		{Kind: "bot", Policy: "bot"}, {Kind: "bot", Policy: "bot"},
	}}
	raw, _ := json.Marshal(req)
	resp, err := http.Post(base+"/api/events", "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create with az-c6 seats: status %d", resp.StatusCode)
	}
	deadline := time.Now().Add(50 * time.Second)
	for {
		resp, err := http.Get(base + "/api/events/e1")
		if err != nil {
			t.Fatal(err)
		}
		var st protocol.EventState
		err = json.NewDecoder(resp.Body).Decode(&st)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if st.Error != "" {
			t.Fatalf("event halted: %s (phase %s)", st.Error, st.Phase)
		}
		for _, s := range st.Seats {
			if s.Drafter != draftpersona.NetPersona {
				t.Fatalf("seat %d drafts with %q, want %q", s.Seat, s.Drafter, draftpersona.NetPersona)
			}
		}
		ended, r1 := 0, 0
		for _, s := range st.Series {
			if s.Round == 1 {
				r1++
				if s.State == "ended" {
					ended++
				}
			}
		}
		if r1 > 0 && ended == r1 {
			for _, s := range st.Seats {
				if s.Picks == 0 {
					t.Fatalf("seat %d drafted nothing", s.Seat)
				}
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("round 1 not ended in 50s (phase %s, %d/%d series ended)", st.Phase, ended, r1)
		}
		time.Sleep(eventsPollEvery)
	}
}
