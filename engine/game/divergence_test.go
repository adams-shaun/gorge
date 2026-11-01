package game_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/replay"
)

// TestDivergenceAlias holds replay's and game's names to one type, so a caller
// matching either finds a divergence from any engine.
func TestDivergenceAlias(t *testing.T) {
	var wrapped error = fmt.Errorf("wrapped: %w", &replay.Divergence{Seq: 3})
	var d *game.Divergence
	if !errors.As(wrapped, &d) || d.Seq != 3 {
		t.Fatalf("errors.As through game.Divergence: %v", d)
	}
	want := []events.Event{{Kind: events.GameStart}}
	if a, b := game.FirstDivergence(want, nil), replay.FirstDivergence(want, nil); a == nil || b == nil || a.Error() != b.Error() {
		t.Fatalf("game says %v, replay says %v", a, b)
	}
}
