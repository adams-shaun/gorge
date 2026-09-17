package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestAnimateColorsFailClosed pins the two Animate fail-closed leaves found
// in review round 2:
//
//  1. A Colors$ value colorLetters cannot fully parse -- the corpus's
//     "ChosenColor" family (25 files: wild_mongrel, kavu_chameleon,
//     dream_coat, ..., every one carrying OverwriteColors$ True) -- must NOT
//     register a colour effect. Registering the parse's empty prefix would
//     overwrite the object to COLOURLESS, an active wrong that replaced the
//     pre-diff passive one; the fail-closed behaviour keeps the printed
//     colours and emits a Note instead.
//  2. Colors$ Colorless WITHOUT OverwriteColors$ (raging_spirit's
//     "{2}: becomes colorless") is an add of the empty set -- a silent no-op
//     whose corpus lines mean "becomes colourless". This build does not
//     implement that grant; it must not register a dead effect and must Note
//     it instead of no-oping silently.
//
// A positive control closes the file: a fully parseable OverwriteColors$
// value still registers exactly one LColor effect and no Note.
//
// These are driven directly (fakeHost + a synthetic SA): no corpus card
// reaches the ChosenColor shape today -- every one of the 25 carriers rides
// an unregistered ChooseColor API that halts the SA chain before the Animate
// so a synthetic SA is the only way to pin the path (the engine-level
// raging_spirit shape is pinned in rules/animate_colors_test.go).
func TestAnimateColorsFailClosed(t *testing.T) {
	animateBoard := func(t *testing.T) (*fakeHost, *Ctx) {
		t.Helper()
		h := newHost(t, 2)
		card := mkCard(t, "Name:Mongrel\nManaCost:1 G\nTypes:Creature Dog\nPT:2/2\nColors:green\nOracle:x\n")
		o := h.g.AddObject(card, 0)
		h.g.Obj(o.ID).Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
		return h, &Ctx{Source: o.ID, Controller: 0}
	}
	notes := func(h *fakeHost) []string {
		var out []string
		for _, ev := range h.log {
			if ev.Kind == events.Note {
				out = append(out, ev.Text)
			}
		}
		return out
	}

	t.Run("ChosenColor fails closed with a Note", func(t *testing.T) {
		h, c := animateBoard(t)
		Resolve(h, c, sa(t, "SP$ Animate | Defined$ Self | Colors$ ChosenColor | OverwriteColors$ True"))
		if len(h.continuous) != 0 {
			t.Fatalf("registered %d continuous effects, want 0 (fail closed, printed colours kept): %+v",
				len(h.continuous), h.continuous)
		}
		var found bool
		for _, n := range notes(h) {
			if strings.Contains(n, "ChosenColor") && strings.Contains(n, "not implemented") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no fail-closed Note among %v", notes(h))
		}
	})

	t.Run("Colorless without OverwriteColors is noted, not registered", func(t *testing.T) {
		h, c := animateBoard(t)
		Resolve(h, c, sa(t, "SP$ Animate | Defined$ Self | Colors$ Colorless"))
		if len(h.continuous) != 0 {
			t.Fatalf("registered %d continuous effects, want 0 (the empty-set add is a no-op): %+v",
				len(h.continuous), h.continuous)
		}
		var found bool
		for _, n := range notes(h) {
			if strings.Contains(n, "Colorless without OverwriteColors$") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no Colorless-without-OverwriteColors Note among %v", notes(h))
		}
	})

	t.Run("parseable OverwriteColors still grants", func(t *testing.T) {
		h, c := animateBoard(t)
		Resolve(h, c, sa(t, "SP$ Animate | Defined$ Self | Colors$ White | OverwriteColors$ True"))
		if len(h.continuous) != 1 {
			t.Fatalf("registered %d continuous effects, want 1: %+v", len(h.continuous), h.continuous)
		}
		ce := h.continuous[0]
		if ce.Layer != state.LColor || ce.OverwriteColors != true ||
			len(ce.AddColors) != 1 || ce.AddColors[0] != "W" {
			t.Fatalf("colour effect = %+v, want one LColor overwrite to [W]", ce)
		}
		if ns := notes(h); len(ns) != 0 {
			t.Fatalf("parseable grant emitted notes %v, want none", ns)
		}
	})
}

// TestColorLettersParseGate pins the colorLetters contract the effAnimate
// gate leans on: every recognised vocabulary word parses, a mixed value with
// one unknown word fails closed as a whole, blank entries neither contribute
// nor spoil the verdict, and "Colorless" parses OK to the EMPTY set (it is
// the caller, not the parser, that decides an empty add is a no-op).
func TestColorLettersParseGate(t *testing.T) {
	cases := []struct {
		in     string
		want   []string
		wantOK bool
	}{
		{"White,Blue", []string{"W", "U"}, true},
		{"All", []string{"W", "U", "B", "R", "G"}, true},
		{"Colorless", nil, true},
		{"", nil, true},
		{" White , Green ", []string{"W", "G"}, true},
		{"ChosenColor", nil, false},
		{"White,ChosenColor", []string{"W"}, false},
	}
	for _, tc := range cases {
		got, ok := colorLetters(tc.in)
		if ok != tc.wantOK {
			t.Errorf("colorLetters(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
		}
		if strings.Join(got, "") != strings.Join(tc.want, "") {
			t.Errorf("colorLetters(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
