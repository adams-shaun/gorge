package gorged

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCreatePodDiracSeatsFour: a server on -engine dirac -seats 4 -format
// commander seats a 4-seat Commander pod of foundations-wretched-ranks, the
// one Commander deck the edh set plays, and the pod plays to its end: three
// bots and a human seat that always passes.
//
// The game's seed is pinned. POST /api/games mints one from crypto/rand, so an
// unpinned run plays a different game every time and how long a four-seat
// heuristic pod takes to end varies with it; a long draw ran past
// testutil.WaitBudget on a loaded CI shard (batch 46121257901, 191 answers).
// Seed 11 is the seed dirac/adapter's TestAdapterFourSeatCommander plays this
// same pod to its end at; pinning it makes this test reproducible. Hunting
// pods that stall belongs to the dirac repro suites, not to this gate.
func TestCreatePodDiracSeatsFour(t *testing.T) {
	oldSeed := newGameSeed
	newGameSeed = func() (uint64, error) { return 11, nil }
	t.Cleanup(func() { newGameSeed = oldSeed })
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testkit", "testutil", "decks", "foundations-wretched-ranks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundations-wretched-ranks.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	c := diracConfig()
	c.diracSets, c.decks, c.seats, c.formatsRaw = "edh", dir, 4, "commander"
	c.vsbot, c.botPolicy = true, "bot"
	base, cancel, served := startServe(t, c)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"format": "commander", "seats": 4, "humans": 1, "bot_policy": "bot"})
	resp, err := http.Post(base+"/api/games", "application/json", bytes.NewReader(body))
	if err != nil {
		select {
		case serr := <-served:
			t.Fatalf("the server exited: %v", serr)
		case <-time.After(time.Second):
		}
		t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create a 4-seat dirac pod: %d %s", resp.StatusCode, out)
	}
	var g httpapi.CreateGameResponse
	if err := json.Unmarshal(out, &g); err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, in any) (int, []byte) {
		var rd io.Reader
		if in != nil {
			b, _ := json.Marshal(in)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, base+"/api/tables/"+g.Table+"/matches/1"+path, rd)
		req.Header.Set("Authorization", "Bearer "+g.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	type option struct {
		Index int    `json:"index"`
		Kind  string `json:"kind"`
	}
	var v struct {
		Over    bool `json:"over"`
		Players []struct {
			Life int `json:"life"`
		} `json:"players"`
	}
	answers := 0
	for deadline := time.Now().Add(testutil.WaitBudget); ; {
		select {
		case err := <-served:
			t.Fatalf("server exited after %d answers: %v", answers, err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the pod has no end after %d answers", answers)
		}
		s, raw := call(http.MethodGet, "/view?seat=0", nil)
		if s == http.StatusNotFound {
			time.Sleep(time.Second / 100)
			continue
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("view %d: %s", s, raw)
		}
		if answers == 0 && len(v.Players) != 4 {
			t.Fatalf("the pod's view seats %d players, want 4", len(v.Players))
		}
		if v.Over {
			break
		}
		s, raw = call(http.MethodGet, "/pending?seat=0", nil)
		var d struct {
			Seq    uint64   `json:"seq"`
			Player int      `json:"player"`
			Kind   string   `json:"kind"`
			Min    int      `json:"min"`
			Opts   []option `json:"options"`
		}
		if s != http.StatusOK || json.Unmarshal(raw, &d) != nil || d.Kind == "" || d.Player != 0 {
			time.Sleep(time.Second / 100)
			continue
		}
		choices := []int{}
		if d.Kind == "priority" {
			for _, o := range d.Opts {
				if o.Kind == "pass" {
					choices = []int{o.Index}
				}
			}
		} else {
			for i := 0; i < d.Min; i++ {
				choices = append(choices, d.Opts[i].Index)
			}
		}
		if s, raw = call(http.MethodPost, "/intent?seat=0", map[string]any{"seq": d.Seq, "player": 0, "choices": choices}); s != http.StatusNoContent {
			t.Fatalf("%s answer: %d %s", d.Kind, s, raw)
		}
		answers++
	}
	if len(v.Players) != 4 {
		t.Fatalf("the finished pod's view seats %d players, want 4", len(v.Players))
	}
	s, raw := call(http.MethodGet, "/events?seat=0", nil)
	var evs []struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(raw, &evs); s != http.StatusOK || err != nil || len(evs) == 0 {
		t.Fatalf("events %d: %v", s, err)
	}
	last := evs[len(evs)-1].Line
	if !strings.Contains(last, "win") && !strings.Contains(last, "draw") {
		t.Fatalf("the pod's log ends on %q, not a result", last)
	}
	t.Logf("the pod ended after %d human answers: %s", answers, last)
}
