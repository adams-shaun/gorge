package eventhost

import (
	"context"
	"sync"

	"github.com/adams-shaun/gorge/server/host"
)

// LazySeries is a host.SeriesFunc holder whose calls park until the real
// target is installed, instead of answering stop. A series table
// goroutine's first act is to ask the hook for match 1's plan, and a
// "no plan" answer ends the table (the registry returns on !ok) with the
// GameStarted already on the log — a wedged event. The Registry and its
// options exist before the eventhost Host (New resumes synchronously and
// can start series tables), so an embedder installs the lazy hook as
// host.Options.Series first and Sets the Host's Series right after New;
// a call landing in that window parks, then runs the real hook. Set is a
// one-shot install.
type LazySeries struct {
	mu    sync.Mutex
	fn    host.SeriesFunc
	ready chan struct{}
}

// NewLazySeries builds an unset LazySeries.
func NewLazySeries() *LazySeries { return &LazySeries{ready: make(chan struct{})} }

// Set installs the target, waking every parked call into it. Call it once,
// after the Host exists.
func (l *LazySeries) Set(fn host.SeriesFunc) {
	l.mu.Lock()
	l.fn = fn
	l.mu.Unlock()
	close(l.ready)
}

// Series runs the installed target. Before Set it parks — the registry may
// block minutes between the games of a series, and a table goroutine here
// is no different — or answers no on a done ctx, per the SeriesFunc
// contract.
func (l *LazySeries) Series(ctx context.Context, t host.TableID, k int) (host.SeriesGame, bool, error) {
	for {
		l.mu.Lock()
		fn := l.fn
		l.mu.Unlock()
		if fn != nil {
			return fn(ctx, t, k)
		}
		select {
		case <-l.ready:
		case <-ctx.Done():
			return host.SeriesGame{}, false, nil
		}
	}
}
