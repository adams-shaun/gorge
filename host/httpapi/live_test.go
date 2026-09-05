package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// TestConcurrentRESTReadsAgainstALiveMatch is controller ruling FL-46 from
// Task 14's review: rest_test.go only ever reads a match that has already
// finished, so "a request never blocks the match loop and the handlers
// are safe for concurrent use" was asserted in comments but never
// demonstrated by a test. Here several goroutines hit GET .../view (no
// seq, so it resolves to whatever the current head is) and GET
// .../events?since=0 concurrently, throughout a real, un-paced, running
// match, and the match must still finish cleanly underneath them.
func TestConcurrentRESTReadsAgainstALiveMatch(t *testing.T) {
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "Table 1", Seats: 4, Decks: []string{"a", "b", "c", "d"}, Seed: 5, Spectator: view.Omniscient}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(r, Options{}))
	t.Cleanup(srv.Close)

	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	// There is a real (if brief) window right after Start where the
	// table's own goroutine has not yet built the first match; wait for
	// it to actually be live before hammering it concurrently.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if ms, err := r.Matches("t1"); err == nil && len(ms) > 0 && ms[0].Events > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("match never became live")
		}
		time.Sleep(time.Millisecond)
	}

	hit := func(path string) string {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			return fmt.Sprintf("%s: %v", path, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("%s: %d %s", path, resp.StatusCode, body)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			return fmt.Sprintf("%s: content-type %q", path, resp.Header.Get("Content-Type"))
		}
		if !json.Valid(body) {
			return fmt.Sprintf("%s: invalid JSON body: %s", path, body)
		}
		return ""
	}

	const rounds = 25
	var wg sync.WaitGroup
	errs := make(chan string, rounds*2)
	for i := 0; i < rounds; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if e := hit("/api/tables/t1/matches/1/view"); e != "" {
				errs <- e
			}
		}()
		go func() {
			defer wg.Done()
			if e := hit("/api/tables/t1/matches/1/events?since=0"); e != "" {
				errs <- e
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}

	r.Wait("t1")
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("match did not finish cleanly: err=%v ms=%+v", err, ms)
	}
}
