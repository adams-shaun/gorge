package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Rikala, Homarid King's gated tap extra played decision by decision against
// gorge (dirac/kgen lower_tapextra.go tapExtraGated): from the third tide
// counter its gate holds, gorge's planner defers every Island of its
// controller (the gated extra is no foldable trigger) and the potential reads
// the tap without the extra, while a tap that happens floats both {U}. The
// kernel mirrors all three: the class line defers under Trigger.Cond
// (mana_tapsdefer.go), the pricing readers skip the Cond2-gated static
// (mana_tapextra.go tapExtraReaches) and addGatedExtra floats it at the tap.
//
// TestTapsForManaRikalaSweep plays a 40-card Rikala deck over 150 seeds on
// both surfaces (seed%4 == 0 on gorge's autopay surface, else the full
// one) and fails on every divergence.
const rikalaSweepSeeds = 150

func rikalaSweepDecks() [2][]string {
	d := deckOf(4, "Rikala, Homarid King")
	d = append(d, deckOf(4, "Opt")...)
	d = append(d, deckOf(3, "Island")...)
	d = append(d, deckOf(3, "Forest")...)
	d = append(d, deckOf(26, "Grizzly Bears")...)
	return [2][]string{d, append(deckOf(8, "Grizzly Bears"), deckOf(32, "Mountain")...)}
}

func TestTapsForManaRikalaSweep(t *testing.T) {
	t.Parallel()
	clean, casts, div := rikalaSweepRun(t, rikalaSweepSeeds, true)
	t.Logf("rikalasweep: %d seeds, %d clean, %d divergent; %d casts/activations", rikalaSweepSeeds, clean, div, casts)
}

// rikalaSweepRun plays seeds 1..n of the Rikala sweep's games; report fails
// the test on each divergence (else the first stops the run) and the return
// is the divergent games.
func rikalaSweepRun(t *testing.T, n uint64, report bool) (clean, casts, div int) {
	t.Helper()
	decks := rikalaSweepDecks()
	reg := synthReg(t)
	set := printedDelveSet(t, decks)
	var gd [2][]*cards.Card
	entries := [2][]cardset.Entry{}
	for s, d := range decks {
		seen := map[string]bool{}
		for _, nm := range d {
			c, ok := reg.Lookup(nm)
			if !ok {
				t.Fatalf("%s not in the corpus", nm)
			}
			gd[s] = append(gd[s], c)
			if !seen[nm] {
				seen[nm] = true
				entries[s] = append(entries[s], cardset.Entry{Name: nm, Count: 1})
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	var mu sync.Mutex
	seedChunks(t, 1, n, lockstepChunk, func(t *testing.T, lo, hi uint64, m *sync.Mutex) {
		c, cs, dv := lockstepSeeds(t, reg, set, "rikalasweep", entries, gd, lo, hi, report, nil, nil, castFirstPolicy)
		mu.Lock()
		clean, casts, div = clean+c, casts+cs, div+dv
		mu.Unlock()
	})
	return clean, casts, div
}

// TestTapsForManaRikalaDeferralMutation: with the Taps-watcher classification
// dropped (dirac.DebugNoTapsDefer) some sweep seed diverges, proof the pool
// exercises Rikala's deferral of its controller's Islands.
func TestTapsForManaRikalaDeferralMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapsDefer = true
	defer func() { dirac.DebugNoTapsDefer = false }()
	if _, _, div := rikalaSweepRun(t, 40, false); div > 0 {
		return
	}
	t.Fatal("no seed diverged with the Taps-watcher classification dropped: the pool does not exercise the deferral")
}
