package eventhost

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCommanderSearchBotPodCompletes: a bot-only MBC Commander draft whose four
// seats play the closed-list max^n search (az-closed-maxn, at one simulation a
// decision: the test checks the wiring, not the search's strength) drafts,
// builds and plays its pod game to COMPLETE, and the game ends on a chain
// head, not a crash. At 4 simulations the pod game ran past 50 s on loaded CI
// runners twice (merge batches #3629 and #3682, both still in ROUND1).
func TestCommanderSearchBotPodCompletes(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	_, decks, _, err := testutil.CommanderAcceptanceDecks(reg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	if err := azredeal.EnableClosedMaxN(b, 1); err != nil {
		t.Fatal(err)
	}
	p := startGamePair(t, "", nil)
	req := commanderReq()
	for i := range req.Seats {
		req.Seats[i] = protocol.CreateEventSeat{Kind: "bot", Policy: azredeal.PolicyClosedMaxN}
	}
	resp := createOK(t, p.h, req)
	r := runOf(t, p.h, resp.Event)
	r.mu.Lock()
	for s, seat := range r.meta.Seats {
		if seat.Policy != azredeal.PolicyClosedMaxN {
			t.Errorf("seat %d policy %q, want %s", s, seat.Policy, azredeal.PolicyClosedMaxN)
		}
	}
	r.mu.Unlock()
	waitPhase(t, p.h, resp.Event, "COMPLETE", 50*time.Second)
	st := stateOf(t, p.h, resp.Event, "")
	if len(st.Pod) != 1 || st.Pod[0].State != "ended" || st.Pod[0].Table == "" {
		t.Fatalf("pod rows %+v", st.Pod)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ended := 0
	for _, en := range r.ev.Log() {
		if en.Kind == "PodGameEnded" {
			ended++
			t.Logf("pod game: head %q attempts %d", en.Head, r.podAttempts[1])
			if en.Head == "crashed" || en.Head == "" {
				t.Fatalf("the pod game did not finish: head %q", en.Head)
			}
		}
	}
	if ended != 1 {
		t.Fatalf("%d PodGameEnded entries, want 1", ended)
	}
}
