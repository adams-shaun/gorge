package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/observe"
)

// TestNativeCounterRoundTrip pins the reverse of Section 6.10's counter
// normalisation for the engine spellings view.View.Counters carries: the
// upper-case numeric and named kinds, and the title-case CR 122.1b keyword
// counters. Only keyword counters are title case; every other kind is
// upper-case, which a hand-picked table cannot cover (the corpus has far more
// named counters than it listed).
func TestNativeCounterRoundTrip(t *testing.T) {
	for _, kind := range []string{
		"P1P1", "M1M1", "P0P1", "LOYALTY", "CHARGE", "EXPERIENCE", "POISON",
		"TICKET", "SPORE", "KI", "STORAGE", "DEPLETION", "PLAN", "VERSE", "FUSE",
		"FLOOD", "POINT", "TIDE", "ICE", "FLAME", "DOOM", "BOUNTY", "HOUR",
		"GROWTH", "FATE", "PRESSURE", "PLAGUE", "PAGE", "FUNGUS", "BRICK", "WIND",
		"Flying", "First Strike", "Double Strike", "Deathtouch", "Vigilance",
	} {
		if got := nativeCounter(observe.Counter(kind)); got != kind {
			t.Errorf("nativeCounter(Counter(%q)) = %q, want %q", kind, got, kind)
		}
	}
	// The wire form of a keyword counter is snake_case; the title-case engine
	// spelling must still come back.
	if got := nativeCounter("first_strike"); got != "First Strike" {
		t.Errorf("nativeCounter(first_strike) = %q, want First Strike", got)
	}
}
