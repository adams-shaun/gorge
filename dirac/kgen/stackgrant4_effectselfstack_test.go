package kgen

import (
	"strings"
	"testing"
)

// TestStackGrantEffectSelfStackRefuses pins that an Effect-issued stack grant
// naming EffectZone$ Stack over Card.Self refuses by name: the kernel reads an
// EffectZone$ Stack self grant off the spell's own printed abilities
// (stackKwSelfN), never an emblem, so an emblem static of that shape would
// lower with no reader. The Effect param allow-list refuses the key first; the
// refusal in effectEmblemStaticHost backs it for a host that admits it.
func TestStackGrantEffectSelfStackRefuses(t *testing.T) {
	src := "Name:Grant Probe\nManaCost:3 R G\nTypes:Sorcery\n" +
		"A:SP$ Effect | StaticAbilities$ Gr | Duration$ EndOfTurn\n" +
		"SVar:Gr:Mode$ Continuous | Affected$ Card.Self | AffectedZone$ Stack | AddKeyword$ Cascade | EffectZone$ Stack\n"
	lw, _ := stackGrantProbeLowered(t, src)
	var all []string
	for _, r := range lw.cards[0].Refusals {
		all = append(all, r.Reason)
	}
	if got := strings.Join(all, " | "); !strings.Contains(got, "EffectZone") {
		t.Errorf("refusals %q, want a refusal naming EffectZone", got)
	}
	if n := len(lw.cards[0].Statics); n != 0 {
		t.Errorf("%d statics lowered, want none", n)
	}
}
