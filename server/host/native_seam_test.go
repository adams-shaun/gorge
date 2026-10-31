package host_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const nativeSeamGolden = "testdata/native-seam.golden"

// TestNativeWireGoldenThroughSeam pins the native wire byte for byte: a seeded
// game between a scripted seat and the default bot, read over the HTTP API at
// every scripted decision and once at the end. The golden stores a hash per
// response and per event, so it holds no card text, and the sample decks keep
// it independent of the card corpus. NATIVE_SEAM_REGEN=1 rewrites it, for a
// deliberate wire or engine change.
func TestNativeWireGoldenThroughSeam(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	byName := map[string][]*cards.Card{}
	for i, n := range names {
		byName[n] = decks[i]
	}
	r, err := host.New(host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			return host.Deck{Name: name, Cards: byName[name]}, nil
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := host.TableConfig{ID: "seam", Name: "seam", Seats: 2, Decks: names, Seed: 20261030,
		Spectator: view.Omniscient, Humans: []int{0}, AutoMana: true, Mulligans: 1}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("seam"); err != nil {
		t.Fatal(err)
	}
	api := nativeSeamClient{t: t, h: httpapi.NewHandler(r, httpapi.Options{
		Seat: func(*http.Request) (httpapi.SeatClaim, bool) { return httpapi.SeatClaim{Table: "seam", Seat: 0}, true },
	})}

	const match = "/api/tables/seam/matches/1"
	var got []string
	record := func(label string, body []byte) {
		sum := sha256.Sum256(body)
		got = append(got, label+" "+hex.EncodeToString(sum[:8]))
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, 0))
	deadline := time.Now().Add(time.Minute)
	for n := 0; ; {
		if time.Now().After(deadline) {
			t.Fatalf("the game is not over after %d decisions", n)
		}
		var infos []protocol.MatchInfo
		if err := json.Unmarshal(api.get("/api/tables/seam/matches"), &infos); err != nil {
			t.Fatal(err)
		}
		if len(infos) == 1 && infos[0].State != protocol.MatchLive {
			if infos[0].State != protocol.MatchFinished {
				t.Fatalf("match ended %s: %s", infos[0].State, infos[0].Head)
			}
			got = append(got, "head "+infos[0].Head)
			break
		}
		status, pending := api.do(http.MethodGet, match+"/pending?seat=0", nil)
		if status != http.StatusOK {
			time.Sleep(time.Millisecond) // the bot is playing, or the match is ending
			continue
		}
		var d decision.Decision
		if err := json.Unmarshal(pending, &d); err != nil {
			t.Fatal(err)
		}
		record(fmt.Sprintf("%d pending", n), pending)
		record(fmt.Sprintf("%d view-seat", n), api.get(match+"/view?seat=0"))
		record(fmt.Sprintf("%d view-spectator", n), api.get(match+"/view"))
		body, err := json.Marshal(scriptedAnswer(&d, rng))
		if err != nil {
			t.Fatal(err)
		}
		if status, resp := api.do(http.MethodPost, match+"/intent", body); status != http.StatusNoContent {
			t.Fatalf("decision %d: intent status %d: %s", n, status, resp)
		}
		n++
	}
	r.Wait("seam")

	record("end view-seat", api.get(match+"/view?seat=0"))
	record("end view-spectator", api.get(match+"/view"))
	for _, q := range []struct{ label, query string }{{"event-seat", "?seat=0"}, {"event-spectator", "?since=0"}} {
		var evs []json.RawMessage
		if err := json.Unmarshal(api.get(match+"/events"+q.query), &evs); err != nil {
			t.Fatal(err)
		}
		if len(evs) == 0 {
			t.Fatalf("%s: no events", q.label)
		}
		for i, ev := range evs {
			record(fmt.Sprintf("%s %d", q.label, i), ev)
		}
	}

	text := strings.Join(got, "\n") + "\n"
	if os.Getenv("NATIVE_SEAM_REGEN") != "" {
		if err := os.MkdirAll(filepath.Dir(nativeSeamGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(nativeSeamGolden, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(nativeSeamGolden)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	for i := range min(len(want), len(got)) {
		if want[i] != got[i] {
			t.Fatalf("native wire moved at %s line %d: want %q, got %q", nativeSeamGolden, i+1, want[i], got[i])
		}
	}
	if len(want) != len(got) {
		t.Fatalf("native wire moved: golden has %d lines, the game produced %d", len(want), len(got))
	}
}

// scriptedAnswer plays any option but concede at a priority decision, chosen
// by rng so casts and activations reach the wire, and the first legal choices
// at every other decision.
func scriptedAnswer(d *decision.Decision, rng *rand.Rand) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}
	if d.Kind == decision.KPriority {
		var offered []int
		for _, o := range d.Options {
			if o.Kind != "concede" {
				offered = append(offered, o.Index)
			}
		}
		in.Choices = append(in.Choices, offered[rng.IntN(len(offered))])
		return in
	}
	for i := 0; i < d.Min && i < len(d.Options); i++ {
		in.Choices = append(in.Choices, d.Options[i].Index)
	}
	return in
}

// nativeSeamClient calls the HTTP API in process, with no socket.
type nativeSeamClient struct {
	t *testing.T
	h http.Handler
}

func (c nativeSeamClient) do(method, path string, body []byte) (int, []byte) {
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(body)))
	return rec.Code, rec.Body.Bytes()
}

func (c nativeSeamClient) get(path string) []byte {
	c.t.Helper()
	status, body := c.do(http.MethodGet, path, nil)
	if status != http.StatusOK {
		c.t.Fatalf("GET %s: status %d: %s", path, status, body)
	}
	return body
}
