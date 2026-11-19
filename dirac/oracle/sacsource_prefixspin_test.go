package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A sacrifice-cost mana ability (Chromatic Star: "sacrifice Chromatic Star:
// add one mana of any colour") runs resolvePrefixStack's ctlPriority arm,
// which before each s.pass(k) wrote s.passes = 1 -- "the other player passing
// as the search assumed", a two-seat reading. At four seats pass() only
// rotates priority when passes < aliveCount(), so the arm reset passes to 1
// every round and the stack never resolved: resolvePrefixStack spun inside a
// single Apply and hung the pod run to the watchdog. The fix passes
// aliveCount()-1 -- the OTHER seats' assumed passes -- which is identical to
// 1 at two seats. The deck carries no Dig carrier, so no Debug* flag has a
// mutation effect to read; only the clean arm is dealt (a mutant run would
// exercise nothing this class owns).

// sacSourcePodDeck: Chromatic Star in the 60, no dig card. Ornithopter gives
// the zero-cost body to cast; the Star's death draw trigger (its sacrifice
// queues it mid-prefix) is the interaction that reaches the prefix stack.
var sacSourcePodDeck = []cardset.Entry{
	{Name: "Chromatic Star", Count: 6}, {Name: "Ornithopter", Count: 6},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
}

// TestFourSeatSacSourcePrefixLockstep: four-seat games of a deck carrying a
// sacrifice-cost mana source play in lockstep; the prefix no longer spins.
func TestFourSeatSacSourcePrefixLockstep(t *testing.T) {
	corpusDirOrSkip(t) // the watch assertions below need the pod to have played
	names := map[string]bool{"Chromatic Star": true}
	hits := map[string]int{}
	var noMut bool // runPodClass flips and restores *mut; this deck reads it never
	st := runPodClass(t, sacSourcePodDeck, 24, remRevealWatch(names, hits), &noMut, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watched decisions", st.games, st.divs, st.stalls, st.hits)
	if st.divs != 0 {
		t.Errorf("%d games diverged", st.divs)
	}
	if st.stalls != 0 {
		t.Errorf("%d games stalled", st.stalls)
	}
	if hits["Chromatic Star"] < 10 {
		t.Errorf("only %d decisions with Chromatic Star on the stack, want >= 10", hits["Chromatic Star"])
	}
}
