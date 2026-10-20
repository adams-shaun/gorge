package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestConcurrentIdenticalIntentPostsOne204Rest409 pins D2 at the HTTP edge:
// duplicate POSTs of the same intent must not hang a handler. The seat claims
// the pending slot atomically, so exactly one POST is admitted and gets the
// engine's 204; every other one is refused as stale (409) instead of waiting
// for a verdict only the winner can receive.
func TestConcurrentIdenticalIntentPostsOne204Rest409(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, _ := parkedSeat(t, r)
	d, err := r.Pending("t1", 1, parked)
	if err != nil {
		t.Fatalf("pending for parked seat %d: %v", parked, err)
	}
	body, err := json.Marshal(answerFor(d))
	if err != nil {
		t.Fatal(err)
	}
	url := srv.URL + "/api/tables/t1/matches/1/intent"
	session := "s" + strconv.Itoa(int(parked))

	const n = 8
	client := &http.Client{Timeout: 10 * time.Second}
	statuses := make([]int, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req, rerr := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			if rerr != nil {
				errs[i] = rerr
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Seat-Session", session)
			resp, derr := client.Do(req)
			if derr != nil {
				errs[i] = derr
				return
			}
			statuses[i] = resp.StatusCode
			resp.Body.Close()
		}(i)
	}
	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("duplicate intent POSTs hung: a handler waited for a verdict it could never receive")
	}

	ok, conflict := 0, 0
	for i := range statuses {
		if errs[i] != nil {
			t.Fatalf("POST %d failed: %v (a duplicate must be refused, not hang or error)", i, errs[i])
		}
		switch statuses[i] {
		case http.StatusNoContent:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("POST %d status %d, want 204 or 409", i, statuses[i])
		}
	}
	if ok != 1 || conflict != n-1 {
		t.Fatalf("statuses = %v: want exactly one 204 and %d 409s", statuses, n-1)
	}
}
