package adapter

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
)

var _ game.Engine = (*Engine)(nil)

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
		return nil, fmt.Errorf("dirac: tables or adapter changed: recorded %s, have %s", c.Key, g.EngineKey())
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
