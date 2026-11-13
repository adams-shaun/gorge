package kgen

import (
	"strings"
	"testing"
)

const drawRevealScript = `Name:Test Seer
ManaCost:1 G
Types:Creature Human Shaman
PT:1/1
A:AB$ Draw | Cost$ T | NumCards$ 1 | Reveal$ All | RememberDrawn$ True | SpellDescription$ Draw a card and reveal it.
Oracle:{T}: Draw a card and reveal it.
`

// TestDrawRevealLowers: Draw's Reveal$ (Fa'adiyah Seer, Sindbad) is read by no
// part of gorge's compileDraw and its draw emits no reveal event, so kgen
// consumes it and the draw lowers as a plain remembering Draw.
func TestDrawRevealLowers(t *testing.T) {
	if r := lowerScript(t, "Test Seer", drawRevealScript); len(r) != 0 {
		t.Fatalf("Draw with Reveal$ refused: %v", r)
	}
	for _, v := range []string{"True", "All"} {
		if r := lowerScript(t, "Test Seer", strings.Replace(drawRevealScript, "Reveal$ All", "Reveal$ "+v, 1)); len(r) != 0 {
			t.Errorf("Reveal$ %s refused: %v", v, r)
		}
	}
}

const spellReturnScript = `Name:Test Ruse
ManaCost:U U
Types:Instant
A:SP$ Counter | Cost$ U U Return<1/Creature> | ValidTgts$ Card | TargetType$ Spell | Destination$ Graveyard | SpellDescription$ Probe.
Oracle:Probe.
`

// TestSpellReturnCostLowers: an instant or sorcery whose Cost$ is its printed
// mana plus exactly one Return<N/Spec> lowers (Familiar's Ruse); a second
// pick part beside it stays refused (one pick cost per cast).
func TestSpellReturnCostLowers(t *testing.T) {
	if r := lowerScript(t, "Test Ruse", spellReturnScript); len(r) != 0 {
		t.Fatalf("Familiar's Ruse shape refused: %v", r)
	}
	if r := lowerScript(t, "Test Ruse", strings.Replace(spellReturnScript, "Types:Instant", "Types:Sorcery", 1)); len(r) != 0 {
		t.Errorf("sorcery refused: %v", r)
	}
	for _, tc := range []struct{ name, cost string }{
		{"a sacrifice beside it", "U U Return<1/Creature> Sac<1/Land>"},
		{"a discard beside it", "U U Return<1/Creature> Discard<1/Card>"},
		{"life beside it", "U U Return<1/Creature> PayLife<2>"},
	} {
		r := lowerScript(t, "Test Ruse", strings.Replace(spellReturnScript, "Cost$ U U Return<1/Creature>", "Cost$ "+tc.cost, 1))
		if len(r) == 0 || !strings.Contains(strings.Join(r, "\n"), "return cost on a spell") {
			t.Errorf("%s: refusals %v, want one naming the return cost", tc.name, r)
		}
	}
}
