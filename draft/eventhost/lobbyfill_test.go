package eventhost

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestLobbyTimeoutFillsPartialLobbyWithBots covers the lobby rule:
// an abandoned lobby where at least one human joined does NOT cancel —
// every unjoined human seat becomes a bot (default persona, the host's
// default game policy, named like a created bot), the event starts and the
// stale join token of the converted seat no longer authenticates. The
// zero-joined cancel is TestLobbyTimeoutCancelsAndFreesSlot.
func TestLobbyTimeoutFillsPartialLobbyWithBots(t *testing.T) {
	clock := newTestClock()
	h, _ := newHost(t, func(o *Options) { o.Now = clock.Now })
	req := oneHumanReq()
	req.Seats[1] = protocol.CreateEventSeat{Kind: "human"}
	resp := createOK(t, h, req)
	id := resp.Event
	joinOK(t, h, id, inviteToken(t, resp, 0), "Ann")
	// Seat 1 never joins; seat 0 must not be able to start the event alone.
	if st := stateOf(t, h, id, ""); st.Phase != "LOBBY" {
		t.Fatalf("phase before the window closes: %s", st.Phase)
	}

	// The precondition the fill turns on: seat 1 is an unjoined human.
	r := runOf(t, h, id)
	r.mu.Lock()
	preKind, preName := r.meta.Seats[1].Kind, r.meta.Seats[1].Name
	r.mu.Unlock()
	if preKind != "human" || preName != "" {
		t.Fatalf("precondition: seat 1 = %q/%q, want unjoined human", preKind, preName)
	}

	clock.Add(15*time.Minute + time.Minute)
	h.checkTimeouts(clock.Now())

	if st := stateOf(t, h, id, ""); st.Phase != "DRAFT" {
		t.Fatalf("phase after the fill sweep = %s, want DRAFT (filled, not cancelled)", st.Phase)
	}
	st := stateOf(t, h, id, "")
	if st.Seats[0].Kind != "human" || st.Seats[0].Name != "Ann" || !st.Seats[0].Joined {
		t.Fatalf("joined human seat mutated: %+v", st.Seats[0])
	}
	if st.Seats[1].Kind != "bot" || st.Seats[1].Name == "" || !st.Seats[1].Joined {
		t.Fatalf("filled seat 1 = %+v, want a joined bot with a name", st.Seats[1])
	}

	r.mu.Lock()
	meta := r.meta.Seats[1]
	created := event.Entry{}
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindEventCreated {
			created = en
		}
	}
	r.mu.Unlock()
	if meta.Kind != "bot" || meta.TokenSHA256 != "" {
		t.Fatalf("meta seat 1 after fill = %+v, want a bot with no invite token", meta)
	}
	if meta.Policy != "bot" || meta.Drafter != "rating-colour" {
		t.Fatalf("filled seat's persona = %s/%s, want rating-colour/bot", meta.Drafter, meta.Policy)
	}
	if len(created.Seats) != 4 {
		t.Fatalf("EventCreated seats = %d, want 4", len(created.Seats))
	}
	if created.Seats[1].Kind != "bot" || created.Seats[1].Persona != "rating-colour/bot" {
		t.Fatalf("EventCreated seat 1 = %+v, want a logged bot", created.Seats[1])
	}

	// The converted seat's stale join token is dead: it authenticates
	// nothing any more.
	_, err := h.EventState(id, inviteToken(t, resp, 1))
	wantErr(t, err, codeUnauthorized, 401)
}
