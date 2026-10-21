package bot

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
)

// TestBotDefaultMulliganIsForge pins FORGE-M1: the production bot answers
// London mulligans with Forge's keep/bottom rule in both mana modes.
func TestBotDefaultMulliganIsForge(t *testing.T) {
	for _, auto := range []bool{false, true} {
		got := Base(bots.Options{Seed: 1, AutoPayMana: auto}).MulliganRule()
		if got != botpolicy.MulliganForge {
			t.Fatalf("auto=%v: Base mulligan rule = %s, want forge", auto, got)
		}
	}
}
