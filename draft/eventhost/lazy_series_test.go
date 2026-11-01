package eventhost

import (
	"context"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
)

// TestLazySeriesParksUntilSet: an unset LazySeries parks a call until Set
// (a "no plan" answer there would end the caller's series table), Set
// wakes it into the target, and an unset one answers no on a done ctx.
func TestLazySeriesParksUntilSet(t *testing.T) {
	l := NewLazySeries()
	type answer struct {
		g  host.SeriesGame
		ok bool
	}
	got := make(chan answer, 1)
	go func() {
		g, ok, err := l.Series(context.Background(), "t", 1)
		if err != nil {
			t.Errorf("parked call errored: %v", err)
		}
		got <- answer{g, ok}
	}()

	// Precondition: with no target the call must park, not answer.
	select {
	case a := <-got:
		t.Fatalf("unset LazySeries answered %+v ok=%v instead of parking", a.g, a.ok)
	case <-time.After(parkDwell): // dwell window, see parkDwell
	}
	l.Set(func(_ context.Context, _ host.TableID, k int) (host.SeriesGame, bool, error) {
		return host.SeriesGame{Chooser: k}, true, nil
	})
	select {
	case a := <-got:
		if !a.ok || a.g.Chooser != 1 {
			t.Fatalf("parked call answered %+v ok=%v, want Chooser 1", a.g, a.ok)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Set never woke the parked call")
	}

	// A done ctx answers no without parking — on an still-unset instance.
	l2 := NewLazySeries()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if g, ok, err := l2.Series(ctx, "t", 1); ok || err != nil || g.Chooser != 0 {
		t.Fatalf("unset LazySeries on done ctx = %+v ok=%v err=%v", g, ok, err)
	}
}
