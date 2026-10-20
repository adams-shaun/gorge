package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestDecisionsRouteIsReviewOnly: the decision index route exists only on a
// handler whose Options.Decisions is set (gorged review). A handler without
// it -- every live server -- answers the /api/ catch-all's JSON 404, for
// every method, exactly as for a path it never served; with it, GET serves
// the index, another method is a 405, and the host's not-found is a 404.
func TestDecisionsRouteIsReviewOnly(t *testing.T) {
	t.Parallel()
	reg, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, errors.New("none") }})
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	do := func(h http.Handler, method, path string) (int, []byte) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec.Code, rec.Body.Bytes()
	}
	const path = "/api/tables/t1/matches/1/decisions"

	live := NewHandler(reg, Options{})
	for _, method := range []string{"GET", "POST"} {
		code, body := do(live, method, path)
		var eb protocol.ErrorBody
		_ = json.Unmarshal(body, &eb)
		if code != http.StatusNotFound || eb.Message != "no such endpoint" {
			t.Fatalf("live %s %s = %d %s, want the catch-all 404", method, path, code, body)
		}
	}

	var asked []string
	review := NewHandler(reg, Options{Decisions: func(tid host.TableID, k int) ([]protocol.ReviewDecision, error) {
		asked = append(asked, string(tid))
		if tid != "t1" || k != 1 {
			return nil, host.ErrNotFound
		}
		return []protocol.ReviewDecision{{Intent: 0, AskSeq: 5, MadeSeq: 6, Kind: "priority", Chosen: []int{0},
			Options: []protocol.ReviewOption{{Index: 0, Label: "Pass priority", Kind: "pass"}}, OptionsTotal: 1, Made: "priority:[0]"}}, nil
	}})
	code, body := do(review, "GET", path)
	var ds []protocol.ReviewDecision
	if code != http.StatusOK || json.Unmarshal(body, &ds) != nil || len(ds) != 1 || ds[0].AskSeq != 5 || ds[0].Options[0].Label != "Pass priority" {
		t.Fatalf("review GET = %d %s", code, body)
	}
	if code, body := do(review, "POST", path); code != http.StatusMethodNotAllowed {
		t.Fatalf("review POST = %d %s, want 405", code, body)
	}
	if code, body := do(review, "GET", "/api/tables/t2/matches/1/decisions"); code != http.StatusNotFound {
		t.Fatalf("review GET of another table = %d %s, want 404", code, body)
	}
	if code, body := do(review, "GET", "/api/tables/t1/matches/x/decisions"); code != http.StatusBadRequest {
		t.Fatalf("review GET bad match = %d %s, want 400", code, body)
	}
	if len(asked) != 2 {
		t.Fatalf("Decisions called for %v", asked)
	}
}
