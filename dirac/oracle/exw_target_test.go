package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The exiled-with target lockstep: Nyla, Shirshu Sleuth's ETB exiles up to
// one creature card from your graveyard (the forward link a ChangeZone
// writes) and her end-step trigger, with no Clue in play, returns a TARGET
// card exiled with her to its owner's hand (ValidTgts$ Card.ExiledWithSource:
// the link is read at target time against the trigger's source, then again at
// the resolution recheck; CR 406.6 / 607.2a). Cremate exiles cards from a
// graveyard that the target read must reject; Doom Blade and the vanilla
// creatures feed the graveyard. Seeds alternate the autopay surface.
var kexwTargetPair = kexwPair{name: "nyla-target", decks: [2][]cardset.Entry{
	{
		{Name: "Nyla, Shirshu Sleuth", Count: 6}, {Name: "Cremate", Count: 3}, {Name: "Doom Blade", Count: 3},
		{Name: "Gravedigger", Count: 2}, {Name: "Walking Corpse", Count: 6}, {Name: "Vampire Bats", Count: 4},
		{Name: "Swamp", Count: 16},
	},
	{
		{Name: "Nyla, Shirshu Sleuth", Count: 5}, {Name: "Cremate", Count: 4}, {Name: "Doom Blade", Count: 3},
		{Name: "Gravedigger", Count: 2}, {Name: "Walking Corpse", Count: 7}, {Name: "Vampire Bats", Count: 3},
		{Name: "Swamp", Count: 16},
	},
}}

// The same decks with Dawn-Blessed Pennant swapped in for Vampire Bats: its
// "target card of the chosen type" sets the prep-wide chosenCardTgt, which
// must not send Nyla's ExiledWithSource target back to a link-blind read.
var kexwTargetChosenPair = func() kexwPair {
	p := kexwPair{name: "nyla-target-chosen"}
	for i, d := range kexwTargetPair.decks {
		for _, e := range d {
			if e.Name == "Vampire Bats" {
				e = cardset.Entry{Name: "Dawn-Blessed Pennant", Count: e.Count}
			}
			p.decks[i] = append(p.decks[i], e)
		}
	}
	return p
}()

func TestExwTargetLockstep(t *testing.T) {
	exwTargetLockstep(t, "exwtarget", kexwTargetPair)
}

func TestExwTargetChosenLockstep(t *testing.T) {
	exwTargetLockstep(t, "exwtargetchosen", kexwTargetChosenPair)
}

func exwTargetLockstep(t *testing.T, pool string, pair kexwPair) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(pool)
	if err != nil {
		t.Fatal(err)
	}
	tot := kexwTotals{ByCard: map[string][2]int{}}
	div, stalls := kexwPlayPair(t, reg, set, pair, 1, 200, true, &tot)
	if div {
		return // kexwPlayPair reported the divergence
	}
	t.Logf("200 games, exw {Recorded:%d Reads:%d Matched:%d Rejected:%d}, by card %v",
		tot.Recorded, tot.Reads, tot.Matched, tot.Rejected, tot.ByCard)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	// the preconditions the clean run depends on: Nyla's exile recorded the
	// link, the end-step target read matched it, and a read rejected a card
	// another source exiled
	if r := tot.ByCard["Nyla, Shirshu Sleuth"]; r[0] < 20 || r[1] < 20 {
		t.Errorf("Nyla, Shirshu Sleuth: %d links recorded, %d reads matched; want at least 20 each", r[0], r[1])
	}
	if tot.Rejected == 0 {
		t.Error("no read rejected a card another source exiled")
	}
}
