package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 612.1 / 612.2: Whim of Volrath changes the text of a basic Mountain
// (its type word) until end of turn. The kernel holds no per-permanent text,
// so it stalled (StallTextChange, unmodelled) where gorge rewrote the land.
// The card is therefore refused by name, which leaves its lowering and its
// tail pool whole; the recording is a game that stalled on the read.
func TestMBCTail02ReprosLockstep(t *testing.T) {
	t.Parallel()
	const card = "Whim of Volrath"
	r, err := ReadRepro(filepath.Join("testdata", "repro", "text_change_reads_target_word.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dealt := false
	for _, d := range r.Cards {
		for _, n := range d {
			dealt = dealt || n == card
		}
	}
	if !dealt {
		t.Fatalf("precondition: the recording deals no %s", card)
	}
	set := globSet(t, r.Pool, r)
	i, ok := set.Lookup(card)
	if !ok {
		t.Fatalf("precondition: %s is not in the %s pool", card, r.Pool)
	}
	if st := set.Status(i); st != cardset.StatusRefused {
		t.Fatalf("%s status = %v in %s, want refused: its text change stalls the kernel (textchange) when the target reads the word", card, st, r.Pool)
	}
	// The recording still replays to the declared kernel stall, so the refusal
	// is pinned to the game that showed it rather than to a name alone.
	stdReproNamed(t, "text_change_reads_target_word")
}
