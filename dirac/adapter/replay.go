package adapter

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
)

var _ game.Engine = (*Engine)(nil)

// ErrTablesMoved is a replay of a recording built from other tables, or read by
// another version of this adapter.
var ErrTablesMoved = errors.New("tables moved since this was recorded")

// ForKey is an engine for a recording with this EngineKey: its set and surface.
func ForKey(key string, reg *cards.Registry) (*Engine, error) {
	parts := strings.Split(key, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("dirac: engine key %q names no set and surface", key)
	}
	surface := dirac.SurfaceFull
	if parts[1] == surfaceName(dirac.SurfaceAutopay) {
		surface = dirac.SurfaceAutopay
	}
	return New(parts[:1], reg, surface)
}

// Replay rebuilds a finished recording from its intents, comparing the events each
// one derives with those recorded.
func (e *Engine) Replay(l *events.Log, c game.Config) (game.Game, error) {
	if l == nil {
		return nil, fmt.Errorf("dirac: nil log")
	}
	g, err := e.Walk(l, c, len(l.Intents), nil)
	if err != nil {
		return g, err
	}
	got := g.Log().Events
	if len(got) != len(l.Events) {
		return g, game.FirstDivergence(l.Events, got)
	}
	return g, nil
}

// ReplayTo rebuilds a recording through its first n intents.
func (e *Engine) ReplayTo(l *events.Log, c game.Config, n int) (game.Game, error) {
	return e.Walk(l, c, n, nil)
}

// Walk rebuilds a recording through its first n intents, calling visit at each intent
// boundary and once at the last. Each intent is submitted as recorded: a game that
// moved underneath the log refuses it, or shows in the comparison.
func (e *Engine) Walk(l *events.Log, c game.Config, n int, visit func(g game.Game, i int) error) (game.Game, error) {
	if l == nil {
		return nil, fmt.Errorf("dirac: nil log")
	}
	n = max(0, min(n, len(l.Intents)))
	c.Seed = l.Seed
	gm, err := e.New(c)
	if err != nil {
		return nil, err
	}
	g := gm.(*Game)
	if c.Key != "" && c.Key != g.EngineKey() {
		return nil, fmt.Errorf("dirac: %w: recorded %s, have %s; rebuild with make dirac-tables at the recorded corpus pin", ErrTablesMoved, c.Key, g.EngineKey())
	}
	if err := sameEvents(l, g); err != nil {
		return g, err
	}
	for i := 0; i < n; i++ {
		if visit != nil {
			if err := visit(g, i); err != nil {
				return g, err
			}
		}
		if g.Pending() == nil {
			return g, fmt.Errorf("dirac: no decision pending at intent %d", i)
		}
		if err := g.Submit(l.Intents[i]); err != nil {
			return g, fmt.Errorf("dirac: intent %d rejected: %w", i, err)
		}
		if err := sameEvents(l, g); err != nil {
			return g, err
		}
	}
	if visit != nil {
		if err := visit(g, n); err != nil {
			return g, err
		}
	}
	return g, nil
}

// sameEvents compares what g has derived so far with the recording.
func sameEvents(l *events.Log, g *Game) error {
	got := g.log.Events
	want := l.Events[:min(len(got), len(l.Events))]
	if d := game.FirstDivergence(want, got); d != nil {
		return d
	}
	return nil
}
