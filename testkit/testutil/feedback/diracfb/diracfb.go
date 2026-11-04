// Package diracfb replays a feedback snapshot on whichever engine played it,
// dirac included. It holds feedback's dirac edge apart: feedback is imported
// by engine packages' tests (the snapshots cmd/repro emits into them), and a
// dirac import there would link dirac into those test binaries, so every
// dirac-only change would rebuild and rerun them.
package diracfb

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil/feedback"
)

// GameAt is feedback.EngineAt for a capture on any engine.
func GameAt(t testing.TB, dir string, n int) game.Game {
	t.Helper()
	l, cfg, meta, err := feedback.Load(dir)
	if err != nil {
		t.Fatalf("feedback: %v", err)
	}
	if meta.Engine != adapter.Name {
		return gorgegame.Game{E: feedback.EngineAt(t, dir, n)}
	}
	e, c, err := Dirac(cfg, meta)
	if err != nil {
		t.Fatalf("feedback: %s: %v", dir, err)
	}
	if n < 0 {
		n = len(l.Intents)
	}
	g, err := e.ReplayTo(l, c, n)
	if err != nil {
		t.Fatalf("feedback: %s: replay to intent %d: %v", dir, n, err)
	}
	return g
}

// Dirac is the engine a capture dirac played replays on, and its config.
func Dirac(cfg rules.Config, meta feedback.Meta) (*adapter.Engine, game.Config, error) {
	reg, err := feedback.Corpus()
	if err != nil {
		return nil, game.Config{}, err
	}
	e, err := adapter.ForKey(meta.EngineKey, reg)
	if err != nil {
		return nil, game.Config{}, err
	}
	c := gorgegame.GameConfig(cfg)
	c.Key = meta.EngineKey
	return e, c, nil
}
