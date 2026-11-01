package gorgedevents_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEventsBotOnlyBo3PodCompletesOverHTTP drives a whole all-bot best-of-3
// draft event through the real serve(): created over POST /api/events with
// best_of 3, three rounds of series played on host series tables, COMPLETE
// with the state reporting best_of 3, and the log persisted under
// <dir>/events/e1/ closed with an EventEnded line.
func TestEventsBotOnlyBo3PodCompletesOverHTTP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := startEventsServer(t, []string{"-cards", testutil.CorpusDirOrSkip(t), "-decks", gorgeDecks(t), "-tables", "1", "-seats", "2", "-pace", "0s", "-cooldown", "0s", "-dir", dir, "-spectator", "omniscient", "-seed", "1", "-perpetual=false", "-prewarm=false", "-feedback", "", "-mulligans", "0", "-auto-mana=false", "-bot-auto-mana=false", "-events"})
	req := protocol.CreateEventRequest{Set: "FDN", BestOf: 3, Seats: []protocol.CreateEventSeat{
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

	// The rounds: poll the state until the event is COMPLETE, and catch the
	// state's best_of and series shapes along the way.
	evURL := base + "/api/events/e1"
	var lastState protocol.EventState
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
		lastState = st
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
	if lastState.BestOf != 3 {
		t.Fatalf("state best_of = %d, want 3", lastState.BestOf)
	}
	sawSeries := false
	for _, s := range lastState.Series {
		if s.BestOf != 3 {
			t.Fatalf("series state best_of = %d, want 3: %+v", s.BestOf, s)
		}
		if s.State == "ended" && (s.Score[0] == 2 || s.Score[1] == 2) {
			sawSeries = true
		}
	}
	if !sawSeries || len(lastState.Series) != 6 {
		t.Fatalf("state series = %+v", lastState.Series)
	}

	// The store: the event's log.jsonl exists, carries Sideboarded lines
	// (bots keep their decks between games; a Bo3 event never completes
	// without them) and ends with EventEnded.
	f, err := os.Open(filepath.Join(dir, "events", "e1", "log.jsonl"))
	if err != nil {
		t.Fatalf("event log file: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	last := ""
	sawSideboard := false
	for sc.Scan() {
		last = sc.Text()
		var en struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal([]byte(last), &en); err != nil {
			t.Fatalf("log line %q: %v", last, err)
		}
		if en.Kind == "Sideboarded" {
			sawSideboard = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if !sawSideboard {
		t.Fatal("no Sideboarded line on the event log")
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
