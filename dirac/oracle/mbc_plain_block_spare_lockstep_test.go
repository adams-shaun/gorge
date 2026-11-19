package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1a with Brave the Sands on the plain block path: a blocker may block
// a second attacker, but the kernel's plain offer (blocksCompletable, no
// must-block attacker or lure) counts only blockers in no chosen pair for an
// attacker's floor, so a declared blocker's spare slot may NOT meet a menace
// attacker's second blocker there. The oracle's blockFloorsMatchable once
// counted that spare slot on every KBlockers decision, which offered a pair
// the kernel withheld (Boggart Brute's menace beside Brave the Sands'
// granted second block).
func TestMBCPlainBlockSpareLockstep(t *testing.T) {
	t.Parallel()
	c := mbc5Case{
		name:      "mbc5_brave_the_sands_menace_plain",
		prefer:    []string{"Brave the Sands", "Savannah Lions", "Boggart Brute", "Raging Goblin"},
		decks:     [2][]string{append(deckOf(10, "Boggart Brute", deckOf(10, "Raging Goblin")...), deckOf(20, "Mountain")...), append(deckOf(8, "Brave the Sands", deckOf(12, "Savannah Lions")...), deckOf(20, "Plains")...)},
		seeds:     40,
		attackAll: true,
		blockAll:  true,
		// a blocker of two attackers in one declaration: the plain path still
		// offers a second pair while the declaration stays completable.
		reached: func(g *GorgeSide) bool {
			for i := range g.E.L.Events {
				e := &g.E.L.Events[i]
				if e.Kind != events.DeclareBlockers {
					continue
				}
				seen := map[state.ObjID]int{}
				for _, pr := range e.Pairs {
					if seen[pr[1]]++; seen[pr[1]] == 2 {
						return true
					}
				}
			}
			return false
		},
	}
	games, hits := mbc5Run(t, c)
	t.Logf("%d games, %d reached a two-attacker block", games, hits)
	if hits == 0 {
		t.Fatalf("no game of %d reached a two-attacker block", games)
	}
}
