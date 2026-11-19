package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// ventureChoiceDecks is the venture pod with two basics of each deck swapped
// for the lands whose entry asks: Crossroads Village (an as-enters colour
// choice) and Blood Crypt (a shock land's pay-life election). Runestone
// Caverns' Play may pick either from the exiled cards.
func ventureChoiceDecks() [4][]cardset.Entry {
	decks := venturePodEntries()
	for i := range decks {
		var out []cardset.Entry
		for _, e := range decks[i] {
			if i%2 == 0 && e.Name == "Plains" || i%2 == 1 && e.Name == "Swamp" {
				e.Count -= 6
				out = append(out, e)
				continue
			}
			if e.Name == "Forest" || e.Name == "Island" || e.Name == "Mountain" {
				e.Count -= 6
			}
			out = append(out, e)
		}
		out = append(out, cardset.Entry{Name: "Crossroads Village", Count: 6}, cardset.Entry{Name: "Blood Crypt", Count: 6})
		decks[i] = out
	}
	return decks
}

// ventureChoiceSeeds are the seeds of TestVentureRunestoneChoiceLandLockstep.
var ventureChoiceSeeds = []uint64{2, 5, 8, 12}

// TestVentureRunestoneChoiceLandLockstep plays four-seat venture pods whose
// decks hold Crossroads Village and Blood Crypt: a land Runestone Caverns'
// Play picks is played in pick order, its as-enters colour choice asked with
// the land still in exile and the shock land's election after it, as gorge's
// beginPlay does. No divergence and no stall; both lands must be played from exile.
func TestVentureRunestoneChoiceLandLockstep(t *testing.T) {
	p := newVenturePodOf(t, ventureChoiceDecks())
	asks := 0
	played := map[string]int{}
	for _, seed := range ventureChoiceSeeds {
		res, c, st := p.play(t, seed, 40)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		asks += st.LandAsks
		for n, cnt := range c.exiled {
			played[n] += cnt
		}
		t.Logf("seed %d: %d decisions, over %v %s; %d dungeon(s), %d land ask(s)", seed, res.Decisions, res.Over, res.Stall, c.enters, st.LandAsks)
	}
	if asks == 0 || played["Crossroads Village"] == 0 || played["Blood Crypt"] == 0 {
		t.Fatalf("land plays from exile %v, entry-asking lands played %d: want Crossroads Village and Blood Crypt both played", played, asks)
	}
}
