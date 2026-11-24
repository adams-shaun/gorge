package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The exile->graveyard target lockstep: Bronzebeak Foragers and Gelatinous
// Cube exile a permanent until they leave the battlefield (ChangeZone
// Duration$ UntilHostLeavesPlay, the OpExileUntilLeaves link), then their
// activated abilities put a TARGET card exiled with them into its owner's
// graveyard (Origin$ Exile | Destination$ Graveyard; the target filter reads
// ExiledWithSource at target time). The move is the plain OpReturn with the
// exile origin mask, so a target that left exile is not moved. Zero- and two-mana
// creatures give the exiles a matching mana value for X = 0 and X = 2; each
// deck holds both readers.
//
// What this proves: the readers lower and the duration exile records the link
// on the kernel side with no divergent game. What it does not: it never
// activates the {X} reader, because an {X} activation is offered only from
// floating mana and the kernel diverges at the X ask once a floating source is
// added (the activated-X ticket). The activated move is covered by the kernel
// test TestExileToGraveyardTargetMove and the lowering by kgen's
// TestExileToGraveyardTargetLowers; the read against gorge's link is unproven
// by lockstep.
var kexwTargetGravePair = kexwPair{name: "exwtargetgrave", decks: [2][]cardset.Entry{
	{
		{Name: "Bronzebeak Foragers", Count: 4}, {Name: "Gelatinous Cube", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Memnite", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Bronzebeak Foragers", Count: 4}, {Name: "Gelatinous Cube", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Memnite", Count: 4},
		{Name: "Savannah Lions", Count: 5}, {Name: "Walking Corpse", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
}}

func TestExwTargetGraveLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exwtargetgrave")
	if err != nil {
		t.Fatal(err)
	}
	tot := kexwTotals{ByCard: map[string][2]int{}}
	div, stalls := kexwPlayPair(t, reg, set, kexwTargetGravePair, 1, 200, true, &tot)
	if div {
		return // kexwPlayPair reported the divergence
	}
	t.Logf("200 games, exw {Recorded:%d Reads:%d Matched:%d Rejected:%d}, by card %v",
		tot.Recorded, tot.Reads, tot.Matched, tot.Rejected, tot.ByCard)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	// the precondition the clean run depends on: each reader's duration
	// exile recorded the link on the kernel side (a write-side count, not a
	// comparison against gorge's link; see the pair's comment)
	for _, name := range []string{"Bronzebeak Foragers", "Gelatinous Cube"} {
		if r := tot.ByCard[name]; r[0] < 20 {
			t.Errorf("%s: %d links recorded; want at least 20", name, r[0])
		}
	}
}
