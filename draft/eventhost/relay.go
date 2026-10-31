package eventhost

import (
	"context"
	"sync/atomic"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// Relay lets gorged hand host.Options.OnMatchEnd and Options.Series to an
// eventhost that can only be built after the Registry: the Registry wants
// the relay in its Options, the Host wants the Registry in its own. Next,
// when set, runs first; the targets the Host installs with Set and
// SetSeries run after it.
type Relay struct {
	Next   host.OnMatchEndFunc
	target atomic.Pointer[host.OnMatchEndFunc]
	series atomic.Pointer[host.SeriesFunc]
}

// Set installs the Host's OnMatchEnd as the relay's target.
func (r *Relay) Set(fn host.OnMatchEndFunc) { r.target.Store(&fn) }

// SetSeries installs the Host's Series hook as the relay's target. A
// series table created before the set answers a series plan never
// (eventhost mints the tables itself, so a live series table without the
// hook cannot exist in a gorged process).
func (r *Relay) SetSeries(fn host.SeriesFunc) { r.series.Store(&fn) }

// OnMatchEnd chains Next and the target, returning the first non-nil error.
func (r *Relay) OnMatchEnd(t host.TableID, k int, m protocol.MatchInfo) error {
	if r.Next != nil {
		if err := r.Next(t, k, m); err != nil {
			return err
		}
	}
	if fn := r.target.Load(); fn != nil {
		return (*fn)(t, k, m)
	}
	return nil
}

// Series forwards the series hook to the Host's, once installed. Before
// SetSeries a series table has no plan: the answer is no.
func (r *Relay) Series(ctx context.Context, t host.TableID, k int) (host.SeriesGame, bool, error) {
	if fn := r.series.Load(); fn != nil {
		return (*fn)(ctx, t, k)
	}
	return host.SeriesGame{}, false, nil
}
