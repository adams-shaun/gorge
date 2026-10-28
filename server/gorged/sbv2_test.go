package gorged

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// sbv2State is the JSON body of GET /api/sbv2/v1/tables/{t}/matches/{k}/state.
type sbv2State struct {
	Request *struct {
		RequestType string `json:"request_type"`
		Protocol    string `json:"protocol"`
		RequestID   string `json:"request_id"`
		GameID      string `json:"game_id"`
		Decision    struct {
			ActingSeat string `json:"acting_seat"`
		} `json:"decision"`
	} `json:"request"`
}

type sbv2Error struct {
	Error string `json:"error"`
}

type sbv2Response struct {
	status int
	body   []byte
}

func sbv2Do(t *testing.T, method, url, token string, body []byte) sbv2Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: reading body: %v", method, url, err)
	}
	return sbv2Response{status: resp.StatusCode, body: raw}
}

func sbv2StateOf(t *testing.T, url, token string) (int, sbv2State, sbv2Error) {
	t.Helper()
	res := sbv2Do(t, http.MethodGet, url+"/api/sbv2/v1/tables/t1/matches/1/state", token, nil)
	var st sbv2State
	var e sbv2Error
	if res.status == http.StatusOK {
		if err := json.Unmarshal(res.body, &st); err != nil {
			t.Fatalf("state body %q: %v", res.body, err)
		}
	} else {
		_ = json.Unmarshal(res.body, &e)
	}
	return res.status, st, e
}

// TestSBV2OffMountsNothing mirrors TestManaBrewOffMountsNothing: with -sbv2
// off nothing registers under the v2 prefix, so a request there gets exactly
// the JSON 404 httpapi's own catch-all serves for any unknown /api/ route.
func TestSBV2OffMountsNothing(t *testing.T) {
	t.Parallel()
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, pace: 0, perpetual: false})
	defer cancel()
	waitTables(t, url, 1)

	want := sbv2Do(t, http.MethodGet, url+"/api/definitely-not-a-route", "", nil)
	got := sbv2Do(t, http.MethodGet, url+"/api/sbv2/v1/tables/t1/matches/1/stream", "", nil)
	if got.status != want.status || !bytes.Equal(got.body, want.body) {
		t.Fatalf("off mount: got %d %q, want the catch-all's %d %q", got.status, got.body, want.status, want.body)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestSBV2ProbeUnsupported pins the v2 coverage probe: state answers 409
// {"error":"unsupported"} for a table v2 does not cover -- more than two
// seats, Commander, or manual mana -- instead of serving a request.
func TestSBV2ProbeUnsupported(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  config
	}{
		{"four-seats", config{tables: 1, seats: 4, humansRaw: "0", pace: 0,
			perpetual: false, mulligans: 0, autoMana: true, seatToken: "tok", sbv2: true}},
		{"manual-mana", config{tables: 1, seats: 2, humansRaw: "0", pace: 0,
			perpetual: false, mulligans: 0, autoMana: false, seatToken: "tok", sbv2: true}},
		{"commander", config{tables: 1, seats: 2, humansRaw: "0", pace: 0,
			perpetual: false, mulligans: 0, autoMana: true, formatsRaw: "commander", seatToken: "tok", sbv2: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, cancel, done := startServe(t, tc.cfg)
			defer cancel()
			waitTables(t, url, 1)

			status, _, e := sbv2StateOf(t, url, "tok")
			if status != http.StatusConflict || e.Error != "unsupported" {
				t.Fatalf("state: %d %q, want 409 unsupported", status, e)
			}

			cancel()
			if err := <-done; err != nil {
				t.Fatalf("serve: %v", err)
			}
		})
	}
}

// TestSBV2TokenIsTheSeat pins that the seat comes from the token only: a token
// for seat 1 sees its own (empty) request state, cannot name seat 0's request
// in POST .../send, and leaving seat 0's request untouched.
func TestSBV2TokenIsTheSeat(t *testing.T) {
	t.Parallel()
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, humansRaw: "0,1", pace: 0,
		perpetual: false, mulligans: 1, autoMana: true, botAutoPayMana: true, seatToken: "tok", sbv2: true})
	defer cancel()
	waitTables(t, url, 1)

	// Both seats are human, so they take turns: answer seat 1's own v2
	// requests (candidate 0 is always legal) until seat 0 is the one asked.
	var seat0 *sbv2State
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		status, st, _ := sbv2StateOf(t, url, "tok")
		if status == http.StatusOK && st.Request != nil {
			seat0 = &st
			break
		}
		if status, st1, _ := sbv2StateOf(t, url, "tok-1"); status == http.StatusOK && st1.Request != nil {
			body, err := json.Marshal(map[string]any{"request_id": st1.Request.RequestID, "candidate_id": 0})
			if err != nil {
				t.Fatal(err)
			}
			if res := sbv2Do(t, http.MethodPost, url+"/api/sbv2/v1/tables/t1/matches/1/send", "tok-1", body); res.status != http.StatusNoContent {
				t.Fatalf("seat 1 answer: %d %q", res.status, res.body)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if seat0 == nil {
		t.Fatal("seat 0 never got a v2 request")
	}
	if seat0.Request.Decision.ActingSeat != "p0" {
		t.Fatalf("seat 0's request acts for %q, want p0", seat0.Request.Decision.ActingSeat)
	}

	// Native routes keep working for the same seat: the same parked decision
	// is readable on the native wire while the v2 request is open.
	if res := sbv2Do(t, http.MethodGet, url+"/api/tables/t1/matches/1/pending?seat=0&token=tok", "", nil); res.status != http.StatusOK {
		t.Fatalf("native pending with a v2 request open: %d %q", res.status, res.body)
	}

	// The seat-1 token must not read seat 0's request. It either has no
	// request of its own (the usual state here) or one that acts for p1.
	status, st1, _ := sbv2StateOf(t, url, "tok-1")
	if status != http.StatusOK {
		t.Fatalf("seat 1 state: %d", status)
	}
	if st1.Request != nil && st1.Request.Decision.ActingSeat == "p0" {
		t.Fatalf("seat 1's token read seat 0's request %q", st1.Request.RequestID)
	}

	// The seat-1 token cannot answer seat 0's request.
	body, err := json.Marshal(map[string]any{"request_id": seat0.Request.RequestID, "candidate_id": 0})
	if err != nil {
		t.Fatal(err)
	}
	res := sbv2Do(t, http.MethodPost, url+"/api/sbv2/v1/tables/t1/matches/1/send", "tok-1", body)
	var e sbv2Error
	_ = json.Unmarshal(res.body, &e)
	if res.status != http.StatusConflict || e.Error != "stale" {
		t.Fatalf("seat-1 answer to seat 0's request: %d %q, want 409 stale", res.status, res.body)
	}

	// ...and seat 0's request is still open, unchanged.
	status, again, _ := sbv2StateOf(t, url, "tok")
	if status != http.StatusOK || again.Request == nil || again.Request.RequestID != seat0.Request.RequestID {
		t.Fatalf("seat 0's request was disturbed: %d %+v", status, again.Request)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}
