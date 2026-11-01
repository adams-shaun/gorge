package gorgedevents_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/gorgedevents"
	"github.com/adams-shaun/gorge/server/gorged"

	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// eventsPollEvery is the sleep between polls. It is a poll interval, not a
// threshold: every wait it paces has a deadline of 10s or more, so load only
// slows a poll and never turns PASS into FAIL (the wall-clock ratchet,
// testkit/testutil TestNoNewSubSecondWallClockThresholdInTests).
const eventsPollEvery = time.Second / 20

// startEventsServer serves one config on a loopback port and waits until
// the API answers, the way TestServesTablesOverHTTP does. The server stops
// at cleanup; errc carries serve's return value into the same cleanup.
func startEventsServer(t *testing.T, args []string) string {
	t.Helper()
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- gorged.Serve(ctx, args, ln, gorged.Extensions{Events: gorgedevents.Plugin{}}) }()
	t.Cleanup(func() {
		stop()
		if err := <-done; err != nil && err != context.Canceled {
			t.Errorf("serve: %v", err)
		}
	})
	base := "http://" + ln.Addr().String()
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(base + "/api/tables")
		if err == nil {
			_ = resp.Body.Close()
			return base
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not come up: %v", err)
		}
		time.Sleep(eventsPollEvery)
	}
}

// TestEventsBotOnlyPodCompletesOverHTTP drives a whole all-bot draft event
// through the real serve(): created over POST /api/events, three rounds
// played on host tables, COMPLETE — with the event's log persisted under
// <dir>/events/e1/ and closed with an EventEnded line.
func TestEventsBotOnlyPodCompletesOverHTTP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := startEventsServer(t, []string{"-cards", testutil.CorpusDirOrSkip(t), "-decks", gorgeDecks(t), "-tables", "1", "-seats", "2", "-pace", "0s", "-cooldown", "0s", "-dir", dir, "-spectator", "omniscient", "-seed", "1", "-perpetual=false", "-prewarm=false", "-feedback", "", "-mulligans", "0", "-auto-mana=false", "-bot-auto-mana=false", "-events"})
	req := protocol.CreateEventRequest{Set: "FDN", Seats: []protocol.CreateEventSeat{
		{Kind: "bot", Policy: "bot"}, {Kind: "bot", Policy: "bot"},
		{Kind: "bot", Policy: "lethal-pressure"}, {Kind: "bot", Policy: "forge-heuristic"},
	}}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(base+"/api/events", "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var created protocol.CreateEventResponse
	err = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || created.Event != "e1" {
		t.Fatalf("create: %d event=%q", resp.StatusCode, created.Event)
	}

	// The rounds: poll the state until the event is COMPLETE.
	evURL := base + "/api/events/e1"
	deadline := time.Now().Add(50 * time.Second)
	for {
		resp, err := http.Get(evURL)
		if err != nil {
			t.Fatal(err)
		}
		var st protocol.EventState
		err = json.NewDecoder(resp.Body).Decode(&st)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if st.Phase == "COMPLETE" {
			break
		}
		if st.Error != "" {
			t.Fatalf("event halted: %s (phase %s)", st.Error, st.Phase)
		}
		if time.Now().After(deadline) {
			t.Fatalf("event not COMPLETE in 50s (phase %s)", st.Phase)
		}
		time.Sleep(eventsPollEvery)
	}

	// The store: the event's log.jsonl exists and ends with EventEnded.
	f, err := os.Open(filepath.Join(dir, "events", "e1", "log.jsonl"))
	if err != nil {
		t.Fatalf("event log file: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	last := ""
	for sc.Scan() {
		last = sc.Text()
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	var en struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(last), &en); err != nil {
		t.Fatalf("last log line %q: %v", last, err)
	}
	if en.Kind != "EventEnded" {
		t.Fatalf("last log line kind %q, want EventEnded", en.Kind)
	}
}

// TestEventsOffAre404 pins the off state: a server built exactly like the
// events-on one but without the flag answers every /api/events path with
// the catch-all JSON 404 and never creates the event store directory.
func TestEventsOffAre404(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := startEventsServer(t, []string{"-cards", testutil.CorpusDirOrSkip(t), "-decks", gorgeDecks(t), "-tables", "1", "-seats", "2", "-pace", "0s", "-cooldown", "0s", "-dir", dir, "-spectator", "omniscient", "-seed", "1", "-perpetual=false", "-prewarm=false", "-feedback", "", "-mulligans", "0", "-auto-mana=false", "-bot-auto-mana=false"})
	resp, err := http.Post(base+"/api/events", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("POST /api/events with -events off: %d %s", resp.StatusCode, body)
	}
	var e protocol.ErrorBody
	if err := json.Unmarshal(body, &e); err != nil || e.Code == "" {
		t.Fatalf("404 body %q is not a JSON error", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "events")); !os.IsNotExist(err) {
		t.Fatalf("event store directory exists with -events off: %v", err)
	}
}
