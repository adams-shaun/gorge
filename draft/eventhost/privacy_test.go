package eventhost

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/protocol"
)

// TestSeatResponsesHideOtherSeatsPacks serialises every seat-scoped body
// seat 0 can receive mid-draft and asserts none of seat 1's private card
// ids appear in any of them: a response for seat A never carries seat B's
// pack or pool before COMPLETE.
func TestSeatResponsesHideOtherSeatsPacks(t *testing.T) {
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	p := packOf(t, h, resp.Event, tok)
	if _, err := h.Pick(resp.Event, tok, protocol.PickRequest{
		Round: p.Round, PickNo: p.PickNo, ID: p.Pack[0].ID,
	}); err != nil {
		t.Fatalf("first pick: %v", err)
	}
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	hidden := map[string]bool{}
	if v, ok := r.ev.Next(1); ok {
		for _, c := range v.Pack {
			hidden[c.ID] = true
		}
	}
	for _, ref := range r.ev.Pool(1) {
		hidden[ref.ID] = true
	}
	pack0 := r.packFor(0)
	state0 := r.stateFor(0)
	r.mu.Unlock()
	if len(hidden) == 0 {
		t.Fatal("no seat-1 private ids collected: precondition missing")
	}
	if pack0.Waiting || len(pack0.Pack) == 0 || len(pack0.Pool) == 0 {
		t.Fatalf("seat 0 has no live pack/pool to serialise: waiting=%v pack=%d pool=%d",
			pack0.Waiting, len(pack0.Pack), len(pack0.Pool))
	}
	for _, body := range []struct {
		name string
		b    []byte
	}{
		{"draft_pack", mustJSON(t, pack0)},
		{"event_state", mustJSON(t, state0)},
	} {
		for id := range hidden {
			// Search for the QUOTED id: draft ids are sequential c1..cN, so a
			// raw needle like "c14" prefix-matches seat 0's own "c140" in the
			// JSON body and reports a spurious leak (E1-4 review round 1).
			if strings.Contains(string(body.b), `"`+id+`"`) {
				t.Fatalf("%s for seat 0 leaks seat 1's card %q: %s", body.name, id, body.b)
			}
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return b
}
