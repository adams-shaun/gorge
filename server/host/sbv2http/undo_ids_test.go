//go:build spellbench

package sbv2http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// handIDs is the seat's own hand as x_gorge_view_v1 numbers it.
func handIDs(t *testing.T, sd protocol.SeatDecision) []int {
	t.Helper()
	var payload xview.Payload
	if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &payload); err != nil {
		t.Fatalf("x_gorge_view_v1: %v", err)
	}
	ids := []int{}
	for _, c := range payload.View.Players[0].Hand {
		ids = append(ids, int(c.ID))
	}
	return ids
}

// TestSBV2UndoKeepsHandIDs pins the ids a v2 client knows the seat's own hand
// by across an undo: the decision the undo puts back is posed with the same
// ids it was first posed with. The browser seat finds the card it just played
// back in hand by that id (the seated-game smoke's land), so an undo that
// renumbers the hand leaves the land "gone" while the client undoes on. The
// deeper the decision, the more drawn cards a rebuilt id tracker would key
// afresh.
func TestSBV2UndoKeepsHandIDs(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, depth := range []int{4, 12, 24, 40, 60} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			const seed = 3000
			hub := NewHub()
			rec := &recorder{}
			table := host.TableID(fmt.Sprintf("ids%d", depth))
			_, base := startExtensionServer(t, reg, hub, rec, table, seed)
			ss := hub.seat(seatKey{table: table, match: 1, seat: 0})
			p := &picker{bot: referenceBot(t, seed)}
			n := 0
			doc, _ := awaitExtensionRequest(t, base, table, ss, p, func(d stateBody) bool {
				n++
				return n > depth && d.Request.Decision.Context.Kind == "priority"
			})
			want := handIDs(t, doc.Request.Decision)
			var first xview.Payload
			if err := json.Unmarshal(doc.Request.Decision.Extensions["x_gorge_view_v1"], &first); err != nil {
				t.Fatal(err)
			}
			send := "/api/sbv2/v1/tables/" + string(table) + "/matches/1/send"
			req, _ := ss.snapshot()
			cand, _ := p.pick(req.ask)
			if status, body := postSBV2Send(t, base, send, "tok", doc.Request.RequestID, cand); status != http.StatusNoContent {
				t.Fatalf("answer: %d %q", status, body)
			}
			doc, _ = awaitExtensionRequest(t, base, table, ss, p, func(d stateBody) bool {
				return decodeHumanExt(t, d.Request.Decision).CanUndo
			})
			if status, body := postHumanAction(t, base, send, "tok", doc.Request.RequestID, map[string]bool{"undo": true}); status != http.StatusNoContent {
				t.Fatalf("undo: %d %q", status, body)
			}
			doc, _ = awaitExtensionRequest(t, base, table, ss, p, func(d stateBody) bool {
				var pl xview.Payload
				_ = json.Unmarshal(d.Request.Decision.Extensions["x_gorge_view_v1"], &pl)
				return pl.NativeIndex == first.NativeIndex
			})
			if got := handIDs(t, doc.Request.Decision); !slices.Equal(got, want) {
				t.Fatalf("after the undo decision %d poses the hand as %v, first posed as %v", first.NativeIndex, got, want)
			}
		})
	}
}
