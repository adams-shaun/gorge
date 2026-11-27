package kgen

import (
	"strings"
	"testing"
)

// drawProgramCarriers are the Draw replacements this slice lowers: a
// ReplaceWith$ body composed of a draw, a life change or a token
// (ReplDrawProgram), or a bodyless Prevent$ skip. The optional/choice group
// and every body that moves library or graveyard cards (a dig, a ChooseMove,
// a Play) keep their refusal until their own slice proves the kernel's
// movement against gorge's.
var drawProgramCarriers = []string{
	"Blood Scrivener", "Hullbreacher", "Notion Thief", "Obstinate Familiar", "Possessed Portal",
}

func TestReplDrawReplSlice(t *testing.T) {
	for _, name := range drawProgramCarriers {
		c := lowerNamedCard(t, name)
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "Draw replacement") {
				t.Errorf("%s: Draw replacement still refused: %s", name, r.Reason)
			}
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "repl DrawProgram") {
			t.Errorf("%s: lowered IR lacks the draw program:\n%s", name, sb.String())
		}
	}
}

// The rest of gap-repl-draw-damage's draw half keeps the refusal class its
// shape needs until the follow-up slices lower it.
func TestReplDrawReplRestRefused(t *testing.T) {
	for _, w := range []struct{ name, class string }{
		{"Abundance", "Draw replacement with an optional or choice body"},
		{"Archmage Ascension", "Draw replacement"},
		{"Asmodeus the Archfiend", "Draw replacement"},
		{"Breathstealer's Crypt", "Draw replacement"},
		{"Chains of Mephistopheles", "Draw replacement"},
		{"Enduring Renewal", "Draw replacement"},
		{"Eruth, Tormented Prophet", "Draw replacement"},
		{"Forbidden Crypt", "Draw replacement"},
		{"Out of the Tombs", "Draw replacement"},
		{"Pavel Chekov, First Posting", "Draw replacement with an optional or choice body"},
		{"Pursuit of Knowledge", "Draw replacement"},
		{"Sages of the Anima", "Draw replacement"},
		{"Tomorrow, Azami's Familiar", "Draw replacement"},
		{"Underrealm Lich", "Draw replacement"},
		{"Unpredictable Cyclone", "Draw replacement"},
	} {
		lc := lowerNamedCard(t, w.name)
		if len(lc.Refusals) != 1 {
			t.Errorf("%s refusals = %v, want exactly one", w.name, lc.Refusals)
			continue
		}
		if got := lc.Refusals[0].Reason; !strings.Contains(got, w.class) {
			t.Errorf("%s refusal %q, want it to name %q", w.name, got, w.class)
		}
	}
}
