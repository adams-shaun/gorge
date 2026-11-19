package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Gemstone Caverns lockstep (CR 103.6a; kgen lower_gemstone.go, kernel
// zone_gemstone.go). gorge's opening round (rules/opening_hand.go) asks each
// seat, AliveFrom the starting player and in hand order, whether to begin the
// game with a Gemstone; the starting player is never asked (!PlayFirst). A
// yes puts it onto the battlefield with a luck counter and poses the
// mandatory "opening_exile" pick over the rest of that hand before the next
// ask. The lucky Gemstone then taps for one mana of any colour (the
// Condition-split source, mana_condsplit.go), which is the only green source
// for the pool's Grizzly Bears. Two-seat and four-seat games, both surfaces:
// no divergence and no kernel stop; asking the starting player too
// (DebugLeylineGemUngated) or skipping the exile pick
// (DebugLeylineGemNoExile) diverges on some seed.
func gemstonePlay(t *testing.T, decks [][]string, surface string) func(seed uint64) Result {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("gemstonecaverns")
	if err != nil {
		t.Skip("no gemstonecaverns fixture table: " + err.Error())
	}
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			Seed: seed, PassElision: true, Surface: surface, Mulligans: 1}
		for i := 2; i < len(decks); i++ {
			r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
			r.MoreCards = append(r.MoreCards, decks[i])
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 12
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, RandomPolicy(seed), rc)
	}
}

func gemstoneDecks(n int) [][]string {
	gem := deckOf(10, "Gemstone Caverns", deckOf(12, "Grizzly Bears", deckOf(18, "Mountain")...)...)
	out := make([][]string, n)
	for i := range out {
		out[i] = gem
	}
	return out
}

func TestGemstoneCavernsOpeningHandLockstep(t *testing.T) {
	const seeds = 10
	for _, surface := range []string{"full", "autopay"} {
		play := gemstonePlay(t, gemstoneDecks(2), surface)
		fourSeatAtkClean(t, play, seeds)
		if surface == "full" {
			if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugLeylineGemUngated) {
				t.Fatal("asking the starting player about Gemstone Caverns changed no seed")
			}
			if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugLeylineGemNoExile) {
				t.Fatal("skipping Gemstone Caverns' hand exile changed no seed")
			}
		}
	}
}

func TestGemstoneCavernsOpeningHandFourSeatLockstep(t *testing.T) {
	const seeds = 6
	play := gemstonePlay(t, gemstoneDecks(4), "full")
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugLeylineGemUngated) {
		t.Fatal("asking the starting player about Gemstone Caverns changed no seed")
	}
}
