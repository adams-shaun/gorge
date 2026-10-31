package extern

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
)

// TestBarrierBatchesEveryLiveGame: in barrier mode, three begun games that
// each ask one decide produce exactly one batch of 3; with five games and
// MaxBatch 2 no batch exceeds 2 and the five items all go out.
func TestBarrierBatchesEveryLiveGame(t *testing.T) {
	t.Run("three games one batch of 3", func(t *testing.T) {
		h := newHub()
		obs := &batchObs{}
		a := dialRefagent(t, h, refagent.Config{ObserveBatch: obs.record}, Options{})
		keys := make([]int, 3)
		nums := make([]uint64, 3)
		for i := range keys {
			keys[i] = i + 1
			nums[i] = a.beginGame(keys[i])
		}
		for i := 1; i < len(nums); i++ {
			if nums[i] <= nums[i-1] {
				t.Fatalf("game numbers %v are not distinct and increasing", nums)
			}
		}
		if obs.len() != 0 {
			t.Fatalf("batches observed before any call: %v", obs.got())
		}
		var wg sync.WaitGroup
		errCh := make(chan error, len(keys))
		for i, gk := range keys {
			wg.Add(1)
			go func(gk int, i int) {
				defer wg.Done()
				callGame(a, context.Background(), gk, fmt.Sprintf("g%d.0", nums[i]), errCh)
				a.endGame(gk)
			}(gk, i)
		}
		for range keys {
			wantAnswer(t, errCh, "game call")
		}
		wg.Wait()
		assertObs(t, obs, []int{3}, "barrier of three")
	})
	t.Run("five games maxBatch 2", func(t *testing.T) {
		h := newHub()
		obs := &batchObs{}
		a := dialRefagent(t, h, refagent.Config{ObserveBatch: obs.record}, Options{MaxBatch: 2})
		const n = 5
		keys := make([]int, n)
		for i := range keys {
			keys[i] = i + 1
			a.beginGame(keys[i])
		}
		var wg sync.WaitGroup
		errCh := make(chan error, n)
		for _, gk := range keys {
			wg.Add(1)
			go func(gk int) {
				defer wg.Done()
				callGame(a, context.Background(), gk, fmt.Sprintf("g%d.0", gk), errCh)
				a.endGame(gk)
			}(gk)
		}
		for range keys {
			wantAnswer(t, errCh, "game call")
		}
		wg.Wait()
		got := obs.got()
		total := 0
		for _, b := range got {
			if b > 2 {
				t.Fatalf("observed batch of %d, want <= MaxBatch 2 (batches %v)", b, got)
			}
			total += b
		}
		if total != n {
			t.Fatalf("observed %d items across batches %v, want %d", total, got, n)
		}
	})
}

// TestUnscopedCallFlushesAlone: with no games live, one unscoped call is
// its own one-request game and flushes as a batch of 1.
func TestUnscopedCallFlushesAlone(t *testing.T) {
	obs := &batchObs{}
	a := dialRefagent(t, newHub(), refagent.Config{ObserveBatch: obs.record}, Options{})
	errCh := make(chan error, 1)
	callGame(a, context.Background(), nil, "u1.0", errCh)
	wantAnswer(t, errCh, "unscoped call")
	assertObs(t, obs, []int{1}, "unscoped alone")
}

// TestUnscopedCallWaitsForLiveGame: the barrier holds an unscoped request
// back while a begun game is not calling; the flush fires only when the
// game calls (one batch of 2) or ends (one batch of 1) — an early flush
// would show two batches of 1.
func TestUnscopedCallWaitsForLiveGame(t *testing.T) {
	t.Run("game calls, one batch of 2", func(t *testing.T) {
		h := newHub()
		enq := make(chan struct{}, 8)
		h.onEnqueue = func() { enq <- struct{}{} }
		obs := &batchObs{}
		a := dialRefagent(t, h, refagent.Config{ObserveBatch: obs.record}, Options{})
		gk := 41
		a.beginGame(gk)
		errCh := make(chan error, 2)
		go callGame(a, context.Background(), nil, "u1.0", errCh)
		<-enq // the unscoped request is enqueued
		if n := obs.len(); n != 0 {
			t.Fatalf("%d batches observed while the begun game was not calling: %v", n, obs.got())
		}
		go callGame(a, context.Background(), gk, "g1.0", errCh)
		wantAnswer(t, errCh, "game call")
		wantAnswer(t, errCh, "unscoped call")
		assertObs(t, obs, []int{2}, "barrier with game call")
	})
	t.Run("game ends, one batch of 1", func(t *testing.T) {
		h := newHub()
		enq := make(chan struct{}, 8)
		h.onEnqueue = func() { enq <- struct{}{} }
		obs := &batchObs{}
		a := dialRefagent(t, h, refagent.Config{ObserveBatch: obs.record}, Options{})
		gk := 42
		a.beginGame(gk)
		errCh := make(chan error, 1)
		go callGame(a, context.Background(), nil, "u1.0", errCh)
		<-enq
		if n := obs.len(); n != 0 {
			t.Fatalf("%d batches observed while the begun game was not calling: %v", n, obs.got())
		}
		a.endGame(gk)
		wantAnswer(t, errCh, "unscoped call")
		assertObs(t, obs, []int{1}, "barrier with game end")
	})
}

// TestTwoAgentsNoDeadlock: two sessions share one hub; four games
// alternate calls on agent A then agent B ten times, and the process-wide
// waiting count lets both flush — every call answered, every game ends.
func TestTwoAgentsNoDeadlock(t *testing.T) {
	h := newHub()
	a := dialRefagent(t, h, refagent.Config{}, Options{})
	b := dialRefagent(t, h, refagent.Config{}, Options{})
	const games, rounds = 4, 10
	var wg sync.WaitGroup
	errs := make(chan error, games*rounds*2)
	for g := 0; g < games; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			gk := fmt.Sprintf("game-%d", g)
			a.beginGame(gk)
			defer a.endGame(gk)
			for r := 0; r < rounds; r++ {
				if _, err := a.call(context.Background(), gk, decideItem(fmt.Sprintf("g%d.a%d", g, r))); err != nil {
					errs <- fmt.Errorf("game %d round %d agent A: %w", g, r, err)
					return
				}
				if _, err := b.call(context.Background(), gk, decideItem(fmt.Sprintf("g%d.b%d", g, r))); err != nil {
					errs <- fmt.Errorf("game %d round %d agent B: %w", g, r, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if a.Stats().Dead || b.Stats().Dead {
		t.Error("a session died during the two-agent run")
	}
}

// TestReadyModeAnswersAll: in ready mode eight concurrent games of twenty
// calls all get answers, and no batch exceeds MaxBatch.
func TestReadyModeAnswersAll(t *testing.T) {
	h := newHub()
	obs := &batchObs{}
	a := dialRefagent(t, h, refagent.Config{ObserveBatch: obs.record}, Options{Flush: FlushReady, MaxBatch: 4})
	const games, rounds = 8, 20
	var wg sync.WaitGroup
	errs := make(chan error, games*rounds)
	for g := 0; g < games; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			gk := fmt.Sprintf("ready-%d", g)
			a.beginGame(gk)
			defer a.endGame(gk)
			for r := 0; r < rounds; r++ {
				if _, err := a.call(context.Background(), gk, decideItem(fmt.Sprintf("g%d.r%d", g, r))); err != nil {
					errs <- fmt.Errorf("game %d round %d: %w", g, r, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	total := 0
	for _, b := range obs.got() {
		if b > 4 {
			t.Fatalf("observed batch of %d, want <= MaxBatch 4", b)
		}
		total += b
	}
	if total != games*rounds {
		t.Fatalf("%d items answered across batches, want %d", total, games*rounds)
	}
	if s := a.Stats(); s.Decides != games*rounds {
		t.Errorf("Stats().Decides = %d, want %d", s.Decides, games*rounds)
	}
}
