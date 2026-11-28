package gorged

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestWebPodCreateFixtureIsAccepted pins the body the web panel posts for a
// four-seat Commander pod (web/src/lib/playvsbot.test.ts reads the same file):
// it decodes strictly into the server's request type and seats a 4-seat table
// with one human grant.
func TestWebPodCreateFixtureIsAccepted(t *testing.T) {
	raw, err := os.ReadFile("testdata/web-create-pod.json")
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req httpapi.CreateGameRequest
	if err := dec.Decode(&req); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if req.Seats != 4 || req.Humans != 1 || req.Format != "commander" {
		t.Fatalf("fixture = %+v, want commander, 4 seats, 1 human", req)
	}
	srv, r, _ := podServer(t, config{}, []string{"p1", "p2", "p3", "p4"})
	code, body := podPost(t, srv.URL, string(raw))
	if code != http.StatusOK {
		t.Fatalf("POST /api/games: %d %s", code, body)
	}
	var resp httpapi.CreateGameResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Seats) != 1 || resp.Seats[0].Seat != 0 {
		t.Fatalf("grants = %+v, want one for seat 0", resp.Seats)
	}
	var ms []protocol.MatchInfo
	for deadline := time.Now().Add(testutil.WaitBudget); len(ms) == 0 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		ms, _ = r.Matches(host.TableID(resp.Table))
	}
	if len(ms) != 1 || len(ms[0].Seats) != 4 {
		t.Fatalf("matches = %+v, want one 4-seat match", ms)
	}
}
