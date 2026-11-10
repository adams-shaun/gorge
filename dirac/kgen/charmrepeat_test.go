package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// section is the lowered IR of card name: from its CARD line to the next.
func section(lir, name string) string {
	_, rest, _ := strings.Cut(lir, "CARD "+name+" ")
	sec, _, _ := strings.Cut(rest, "CARD ")
	return sec
}

// TestCharmRepeatLowers: a CanRepeatModes$ charm lowers one block per
// instance a mode can be chosen (Cosmium Confluence: three modes, three
// picks; Moment of Reckoning: two modes, four picks; Season of Loss: a
// five-pawprint budget over modes of 1, 2 and 3 pawprints); a charm whose mode
// counts need more than the mode byte, whose modes outnumber the count byte,
// or whose target slots outnumber the ability's, is refused by name.
func TestCharmRepeatLowers(t *testing.T) {
	names := []string{"Cosmium Confluence", "Moment of Reckoning", "Season of Loss", "Planewide Celebration", "Unite the Coalition", "Hela's Arcanum"}
	lir, ref := lowerCarriers(t, names)
	for _, n := range names[:3] {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
	}
	for n, want := range map[string]string{
		"Planewide Celebration": "mode counts need 12 bits",
		"Unite the Coalition":   "repeatable charm with 5 modes",
		"Hela's Arcanum":        "repeatable charm needs 6 target slots",
	} {
		hit := false
		for _, r := range ref[n] {
			hit = hit || strings.Contains(r, want)
		}
		if !hit {
			t.Errorf("%s: want a refusal containing %q, got %v", n, want, ref[n])
		}
	}
	blocks := func(name string) (n int, sec string) {
		sec = section(lir, name)
		return strings.Count(sec, " Charm C="), sec
	}
	for name, want := range map[string]int{"Cosmium Confluence": 9, "Moment of Reckoning": 8, "Season of Loss": 5 + 2 + 1} {
		if n, _ := blocks(name); n != want {
			t.Errorf("%s: %d mode blocks, want %d (one per instance each mode can be chosen)", name, n, want)
		}
	}
	// Moment's two modes count in three bits each: mode 1 starts at bit 3
	if _, sec := blocks("Moment of Reckoning"); !strings.Contains(sec, fmt.Sprintf(" Charm C=%d ", kop.RepeatCharmC(3, 3, 3, 1))) {
		t.Errorf("Moment of Reckoning: no block for the fourth instance of mode 1 (shift 3, width 3):\n%s", sec)
	}
	// Season of Loss: counts of up to 5, 2 and 1 take 3, 2 and 1 bits
	if _, sec := blocks("Season of Loss"); !strings.Contains(sec, fmt.Sprintf(" Charm C=%d ", kop.RepeatCharmC(5, 1, 0, 2))) {
		t.Errorf("Season of Loss: no block for its three-pawprint mode (shift 5, width 1):\n%s", sec)
	}
}
