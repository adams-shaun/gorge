package oracle

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// stdEDHRepros are recordings of the edh pool's two-seat duels (MTGJSON
// Commander precons), replayed on the table their own decks build (pool
// custom). Each row names the divergence it was recorded at and the cards its
// decks must hold, so a re-recorded game that no longer reaches the case
// fails instead of passing vacuously.
var stdEDHRepros = []struct {
	name, field string
	at          int
	cards       []string
}{
	// Malleable Impostor copies a manifested Inspiring Statuary: CR 707.2 and
	// 707.3, the copy takes the face-down copiable values (a nameless 2/2
	// colourless Creature) and Impostor's own exceptions, not the printed
	// artifact. The kernel's copy layer used to point at the printed card: a
	// 0/0 artifact.
	{"clone_facedown_copy", "Cards[47].Power", 481, []string{"Malleable Impostor", "Inspiring Statuary"}},
	// The same game further on: Tegwyll, Duke of Splendor ("other Faerie
	// creatures you control get +1/+1") reads the blank copy's Faerie
	// exception (CR 707.9b), which the face-down "no subtypes" filter gate
	// used to hide: gorge 3/3, the kernel 2/2.
	{"clone_facedown_copy_faerie_lord", "Cards[47].Power", 602, []string{"Malleable Impostor", "Tegwyll, Duke of Splendor"}},
}

// TestStdEDHReprosLockstep replays every edh recording with no divergence for
// 200 decisions past its script.
func TestStdEDHReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range stdEDHRepros {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			if r.Divergence == nil || r.Divergence.At != c.at || r.Divergence.Field != c.field {
				t.Fatalf("the recording is not the divergence at %d on %s: %+v", c.at, c.field, r.Divergence)
			}
			for _, want := range c.cards {
				if !slices.ContainsFunc(r.Cards[:], func(deck []string) bool { return slices.Contains(deck, want) }) {
					t.Fatalf("the repro's decks do not hold %s", want)
				}
			}
			res := replayStdReproResult(t, reg, "custom", c.name, r.Surface == "full")
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("the kernel stalled: %q", res.Stall)
			}
			if res.Decisions <= c.at {
				t.Fatalf("the game ended at decision %d, before the recorded divergence at %d", res.Decisions, c.at)
			}
		})
	}
}
