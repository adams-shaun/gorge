package host

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestConcurrentIdenticalSubmitsAnswerOnceAndNeverHang pins D2: two POSTs of
// the same intent can arrive at once. The seat claims the pending slot under
// its mutex before sending, so exactly one submit is admitted; the rest are
// refused as stale instead of parking on a verdict only one of them can get.
// Before the claim, the second submit could be admitted too and then wait
// forever for a reply the play loop would never send it.
func TestConcurrentIdenticalSubmitsAnswerOnceAndNeverHang(t *testing.T) {
	r, id := startHumanTable(t)
	d := waitPending(t, r, id, 1, 0)
	in := legalIntent(d)

	const n = 8
	start := make(chan struct{})
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = r.SubmitIntent(id, 1, 0, in)
		}(i)
	}
	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent identical submits hung: a handler waited for a verdict it could never receive")
	}

	accepted := 0
	for _, err := range errs {
		if err == nil {
			accepted++
			continue
		}
		// A loser normally sees the claimed slot ("no pending decision"). A
		// loser descheduled past the winner's acceptance and the seat's next
		// park can instead validate against the NEW decision and see a stale
		// seq; both are refusals, and neither may hang. Accept either so the
		// concurrency assertion does not depend on scheduler timing.
		if !strings.Contains(err.Error(), "no pending decision") && !strings.Contains(err.Error(), "intent seq") {
			t.Fatalf("losing submit error = %q, want the stale-slot or stale-seq refusal", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("%d of %d identical submits were accepted, want exactly 1 (errors: %v)", accepted, n, errs)
	}
}

// TestSubmitClaimsTheSlotBeforeTheAnswerIsConsumed is the deterministic
// seat-level pin for D2. The seat is parked but nothing is awaiting yet, so
// every racing submit reaches the slot check before any answer is consumed.
// Claiming the slot under the mutex admits exactly one; the others are told
// there is no pending decision instead of blocking on a verdict only the
// winner can receive.
func TestSubmitClaimsTheSlotBeforeTheAnswerIsConsumed(t *testing.T) {
	s := NewHumanSeat()
	d := decision.Decision{Seq: 3, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Label: "pass"}}}
	p := s.park(context.Background(), view.View{}, d, nil)
	in := decision.Intent{Seq: 3, Player: 0, Choices: []int{0}}

	const n = 8
	start := make(chan struct{})
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = s.submitAdmittedWait(in, nil, false, true)
		}(i)
	}
	close(start)

	// Let every submit reach the seat's slot check before the answer is
	// consumed; a claimed slot rejects the losers immediately, so the wait
	// costs nothing when the fix is present.
	time.Sleep(100 * time.Millisecond)

	got := make(chan decision.Intent, 1)
	go func() {
		answer, _ := p.await()
		got <- answer
	}()
	select {
	case answer := <-got:
		if answer.Seq != d.Seq {
			t.Fatalf("await got seq %d, want %d", answer.Seq, d.Seq)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("await blocked: no submit was admitted")
	}
	p.reply(nil)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a losing submit hung waiting for a verdict it can never receive")
	}

	accepted := 0
	for _, err := range errs {
		if err == nil {
			accepted++
			continue
		}
		if !strings.Contains(err.Error(), "no pending decision") {
			t.Fatalf("losing submit error = %q, want the stale-slot refusal", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("%d submits were accepted, want exactly 1 (errors: %v)", accepted, errs)
	}
}
