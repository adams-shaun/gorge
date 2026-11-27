package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const drawMoreOpponentProbe = `Name:Probe Draw More Opponent
ManaCost:U
Types:Enchantment
R:Event$ Draw | ActiveZones$ Battlefield | ValidPlayer$ Opponent | ReplaceWith$ ProbeDraw | Description$ probe.
SVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 2
`

// TestDrawReplValidPlayerKeepsBits: a plain "draw N instead" body behind
// ValidPlayer$ Opponent must not become ReplDrawMore, whose kernel walk only
// scans the controller's own battlefield (drawMore): the player bits would be
// dropped and the replacement would apply to the controller's own draws.
func TestDrawReplValidPlayerKeepsBits(t *testing.T) {
	c := lowerScriptCard(t, "Probe Draw More Opponent", drawMoreOpponentProbe)
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals = %v, want none", c.Refusals)
	}
	if len(c.Repls) != 1 {
		t.Fatalf("repls = %d, want one", len(c.Repls))
	}
	r := c.Repls[0]
	if r.Kind != "DrawProgram" {
		t.Fatalf("kind = %q, want DrawProgram (DrawMore only for the controller's own draw)", r.Kind)
	}
	if r.Bits&kop.ReplDrawOpponent == 0 {
		t.Fatalf("bits = %d, want the ReplDrawOpponent bit kept", r.Bits)
	}
}
