package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// drawProgramCarriers are the Draw replacements this slice lowers: a
// ReplaceWith$ body composed of a draw, a life change, a token, a win or a
// library-look Dig (ReplDrawProgram), Out of the Tombs' graveyard-to-battlefield
// return with its Remembered-gated LosesGame, or a bodyless Prevent$ skip. The
// optional/choice group and every other body that moves graveyard cards (a
// different ChooseMove, a Play) keep their refusal until their own slice proves
// the kernel's movement against gorge's.
var drawProgramCarriers = []string{
	"Blood Scrivener", "Hullbreacher", "Jace, Wielder of Mysteries", "Laboratory Maniac",
	"Living Conundrum", "Notion Thief", "Obstinate Familiar", "Ormos, Archive Keeper",
	"Out of the Tombs", "Possessed Portal",
	// The library-look Dig bodies whose choose both engines park at the turn
	// draw (kernel repl_drawbody.go; gorge's turn-draw park).
	"Sages of the Anima", "Tomorrow, Azami's Familiar", "Underrealm Lich",
}

// drawWinCarriers are the carriers whose body is "you win the game instead":
// the lowered IR carries the kernel's win op (LoseGame, C 1) and the
// library-empty gate.
var drawWinCarriers = map[string]bool{"Jace, Wielder of Mysteries": true, "Laboratory Maniac": true}

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
		if drawWinCarriers[name] {
			const gate = "cond=Value(op3,LibrarySizeOf(You),0)"
			if !strings.Contains(sb.String(), "LoseGame") || !strings.Contains(sb.String(), gate) {
				t.Errorf("%s: lowered IR lacks the win op or the empty-library gate:\n%s", name, sb.String())
			}
		}
	}
}

// TestReplDrawReplEmptyLibraryKinds pins the two empty-library-gated lines'
// kinds: Ormos's mandatory ReplaceWith$ carries the DB$ PutCounter body as
// its program, and Living Conundrum's bodyless Prevent$ line is the skip
// (no program). Both meet the kernel's empty-library draw scan
// (drawProgramMeetsEmpty, dirac/repl_drawprogram.go).
func TestReplDrawReplEmptyLibraryKinds(t *testing.T) {
	for _, w := range []struct {
		name  string
		check func(t *testing.T, r lRepl)
	}{
		{"Ormos, Archive Keeper", func(t *testing.T, r lRepl) {
			put := false
			for _, in := range r.Prog {
				if in.Op == "PutCounter" {
					put = true
				}
			}
			if !put {
				t.Fatalf("Ormos's DrawProgram carries no PutCounter body: %+v", r.Prog)
			}
			if r.Bits&kop.ReplDrawSkip != 0 {
				t.Fatalf("Ormos's counter body must not be the skip: bits=%#x", r.Bits)
			}
		}},
		{"Living Conundrum", func(t *testing.T, r lRepl) {
			if r.Bits&kop.ReplDrawSkip == 0 {
				t.Fatalf("the bodyless Prevent$ line lacks the skip bit: bits=%#x", r.Bits)
			}
			if len(r.Prog) != 0 {
				t.Fatalf("the bodyless skip carries a program: %+v", r.Prog)
			}
		}},
	} {
		c := lowerNamedCard(t, w.name)
		var found *lRepl
		for i := range c.Repls {
			if c.Repls[i].Kind == "DrawProgram" {
				found = &c.Repls[i]
			}
		}
		if found == nil {
			t.Fatalf("%s lowered no DrawProgram repl: %+v", w.name, c.Repls)
		}
		w.check(t, *found)
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
		{"Pavel Chekov, First Posting", "Draw replacement with an optional or choice body"},
		{"Pursuit of Knowledge", "Draw replacement"},
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
