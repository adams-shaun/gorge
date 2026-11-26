package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The colour-word rewrite reads only the masks a face's own colour words name
// (dirac/static_colorwords.go), under a live Swirl the Mists:
//
//   - Herald of Kozilek's "Colorless spells you cast cost {1} less" is a
//     cost static of another permanent whose filter names no colour word (its
//     mask is every colour, nonColorless/Colorless). Doom Blade, a spell that
//     does name black, is built while Herald is in play: the static must read
//     Doom Blade as the black spell it is, as gorge does (Herald's text
//     carries no word to change, nor does Doom Blade's face move another
//     source's filter).
//   - White Knight's printed Protection from black is the keyword the
//     rewrite changes (a chosen red reads protection from red).
var colorWordsMaskDecks = [2][]cardset.Entry{
	{
		{Name: "Swirl the Mists", Count: 6}, {Name: "Doom Blade", Count: 6}, {Name: "Herald of Kozilek", Count: 6},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "White Knight", Count: 8}, {Name: "Doom Blade", Count: 6}, {Name: "Northern Paladin", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 10},
	},
}

func TestColorWordsLockstepNonWordMasks(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, colorWordsPool, colorWordsMaskDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Fatalf("seat %d: the mask decks are not playable in the %s pool", s, colorWordsPool)
		}
	}
	before := dirac.ColorWordsStats.Stalls.Load() + dirac.ColorWordsStats.Casts.Load()
	live0 := dirac.ColorWordsStats.Live.Load()
	var mu sync.Mutex
	played := 0
	seedChunks(t, 1, 240, lockstepChunk, func(t *testing.T, lo, hi uint64, _ *sync.Mutex) {
		n := 0
		colorWordsPlay(t, colorWordsMaskDecks, lo, hi, func(seed uint64, res *Result) {
			if err := unmodelledStallError(t.Name(), res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
			n++
		})
		mu.Lock()
		played += n
		mu.Unlock()
	})
	if t.Failed() {
		return
	}
	stalls := dirac.ColorWordsStats.Stalls.Load() + dirac.ColorWordsStats.Casts.Load() - before
	live := dirac.ColorWordsStats.Live.Load() - live0
	t.Logf("%d seeds agree; %d priority grants under a live static, %d stalls", played, live, stalls)
	if stalls != 0 {
		t.Errorf("%d unmodelled stalls", stalls)
	}
	if live == 0 {
		t.Fatal("no priority grant found a live colour-word static: the decks never reach the mechanism")
	}
}
