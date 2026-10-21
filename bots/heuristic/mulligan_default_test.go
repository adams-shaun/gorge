package heuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
)

// TestHeuristicNewBotStaysCoin pins FORGE-M1's scope boundary: the default
// mulligan lives in the bot policy constructor, so the heuristic package's
// constructors keep the coin for research and test bots.
func TestHeuristicNewBotStaysCoin(t *testing.T) {
	if got := NewBot(1).MulliganRule(); got != botpolicy.MulliganCoin {
		t.Fatalf("NewBot mulligan rule = %s, want the coin", got)
	}
}
