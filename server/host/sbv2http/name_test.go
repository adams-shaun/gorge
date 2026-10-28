package sbv2http

// TestSBV2NameDecisionPastTheCap plays seed 2011 (tron's Pithing Needle asks
// for any of 34,929 card names) over the real /api/sbv2 routes and answers the
// name through x_gorge_human_v1.choose_name with a name the 4096-candidate
// window does not contain.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestSBV2NameDecisionPastTheCap(t *testing.T) {
	const seed = 2011
	const token = "name-s0"
	reg := testutil.CorpusRegistry(t)
	hub := NewHub()
	r, err := host.New(parityOptions(reg, hub))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	table := host.TableID("name")
	cfg := parityTableConfig(table, seed)
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/sbv2/v1/tables/", http.StripPrefix("/api/sbv2/v1/tables", NewHandler(r, hub, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		if strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ") != token {
			return SeatClaim{}, false
		}
		return SeatClaim{Table: table, Seat: 0}, true
	}})))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if err := r.Start(table); err != nil {
		t.Fatal(err)
	}

	bot, err := productionBot(seed, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := &picker{bot: bot}
	ss := hub.seat(seatKey{table: table, match: 1, seat: 0})
	statePath := fmt.Sprintf("/api/sbv2/v1/tables/%s/matches/1/state", table)
	sendPath := fmt.Sprintf("/api/sbv2/v1/tables/%s/matches/1/send", table)
	post := func(body map[string]any) int {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		status, resp, err := parityPost(srv.URL+sendPath, token, raw)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("POST %s -> %d %s", raw, status, strings.TrimSpace(string(resp)))
		return status
	}

	deadline := time.Now().Add(30 * time.Second)
	handled := map[string]bool{}
	for time.Now().Before(deadline) {
		if done, _, err := parityMatchState(r, table); err != nil {
			t.Fatal(err)
		} else if done {
			t.Fatal("the game finished without a windowed card-name ask")
		}
		status, body, err := parityGet(srv.URL+statePath, token)
		if err != nil || status != http.StatusOK {
			t.Fatalf("GET state: %d %v %s", status, err, body)
		}
		var doc stateBody
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Request == nil || handled[doc.Request.RequestID] {
			time.Sleep(time.Millisecond)
			continue
		}
		id := doc.Request.RequestID
		req, _ := ss.snapshot()
		if req == nil || fmt.Sprint(req.id) != id {
			time.Sleep(time.Millisecond)
			continue
		}
		if isNativeOnly(doc.Request.Decision) {
			if answerParityNative(r, table, bot) == nil {
				handled[id] = true
			}
			continue
		}
		nd := req.ask.Pose.Native
		if mapping.Route(nd) != "choose/name" || len(nd.Options) <= len(req.ask.Pose.Candidates) {
			cand, miss := p.pick(req.ask)
			if miss != "" {
				if err := r.SubmitIntent(table, 1, 0, p.intent); err != nil {
					t.Fatalf("native fallback: %v", err)
				}
			} else if got := post(map[string]any{"request_id": id, "candidate_id": cand}); got == http.StatusBadRequest {
				t.Fatalf("candidate send for request %s: %d", id, got)
			}
			handled[id] = true
			continue
		}

		// The windowed card-name ask.
		cands := doc.Request.Decision.Candidates
		if len(cands) != mapping.MaxCandidates {
			t.Fatalf("candidates = %d, want %d", len(cands), mapping.MaxCandidates)
		}
		var human struct {
			NativeOnly bool        `json:"native_only"`
			NameDomain *nameDomain `json:"name_domain"`
		}
		if err := json.Unmarshal(doc.Request.Decision.Extensions["x_gorge_human_v1"], &human); err != nil {
			t.Fatal(err)
		}
		if human.NativeOnly {
			t.Fatal("the card-name ask was served native_only")
		}
		if want := (nameDomain{Total: len(nd.Options), Offered: len(cands)}); human.NameDomain == nil || *human.NameDomain != want {
			t.Fatalf("name_domain = %+v, want %+v", human.NameDomain, want)
		}
		if human.NameDomain.Total != 34929 {
			t.Fatalf("name_domain.total = %d, want 34929", human.NameDomain.Total)
		}
		last := nd.Options[len(nd.Options)-1].Label
		for _, c := range req.ask.Pose.Candidates {
			if fmt.Sprint(c.Sem.Fields["value"]) == last {
				t.Fatalf("precondition: %q is inside the candidate window", last)
			}
		}

		if got := post(map[string]any{"request_id": id, "x_gorge_human_v1": map[string]any{"choose_name": "Not A Card"}}); got != http.StatusBadRequest {
			t.Fatalf("unknown name = %d, want 400", got)
		}
		if _, ok := ss.openRequest(req.id); !ok {
			t.Fatal("the request closed after a rejected name")
		}
		if got := post(map[string]any{"request_id": id, "x_gorge_human_v1": map[string]any{"choose_name": "Island", "concede": true}}); got != http.StatusBadRequest {
			t.Fatalf("two actions = %d, want 400", got)
		}
		if got := post(map[string]any{"request_id": id, "x_gorge_human_v1": map[string]any{"choose_name": last}}); got != http.StatusNoContent {
			t.Fatalf("choose_name %q = %d, want 204", last, got)
		}
		if got := post(map[string]any{"request_id": id, "candidate_id": 0}); got != http.StatusConflict {
			t.Fatalf("candidate after choose_name = %d, want 409", got)
		}
		for ; time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			if cur, _ := ss.snapshot(); cur != nil && cur.id != req.id {
				return
			}
			if done, _, _ := parityMatchState(r, table); done {
				return
			}
		}
		t.Fatal("the decision did not move on after choose_name")
	}
	t.Fatal("no windowed card-name ask within 30s")
}
