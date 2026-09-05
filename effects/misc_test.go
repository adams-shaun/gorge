package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// TestRepeatHonoursMaxRepeatAndIsBounded is Task 20's hygiene test for
// effRepeat: the fetched corpus's real cards parameterise Repeat with
// MaxRepeat$, not the RepeatNum$ the OLD M1 implementation read (RepeatNum$
// appears nowhere in the corpus, so every real Repeat degraded to a single
// run). Two things are asserted: MaxRepeat$ actually drives the run count,
// and an absurd/unbounded repeat is clamped to the 1000-run safety cap so a
// malformed script can never spin the engine.
func TestRepeatHonoursMaxRepeatAndIsBounded(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"DBLife": "DB$ GainLife | Defined$ You | LifeAmount$ 1"}
	life := h.Game().Players[c.Controller].Life
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "DBLife", "MaxRepeat": "3"}})
	if h.Game().Players[c.Controller].Life != life+3 {
		t.Fatalf("MaxRepeat 3 ran %d times", h.Game().Players[c.Controller].Life-life)
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "DBLife", "MaxRepeat": "999999"}})
	if got := h.Game().Players[c.Controller].Life - life - 3; got != 1000 {
		t.Fatalf("unbounded repeat ran %d times, want the 1000 cap", got)
	}
}
