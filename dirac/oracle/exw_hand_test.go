package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The exiled-with hand pick lockstep: Roads Go Ever, Ever On's chapter I
// searches up to two basic Plains into exile (the forward link a library
// search writes) and chapters II and III each return one card exiled with
// the Saga to its owner's hand (a hidden ChangeZone pick over
// Card.ExiledWithSource, gorge effHiddenPick's game-wide exile walk; CR
// 607.2a). Journey to Nowhere exiles other cards the pick must reject;
// Cloudshift flickers creatures. Seeds alternate the autopay surface.
var kexwHandPair = kexwPair{name: "roads-hand-pick", decks: [2][]cardset.Entry{
	{
		{Name: "Roads Go Ever, Ever On", Count: 8}, {Name: "Journey to Nowhere", Count: 3}, {Name: "Cloudshift", Count: 3},
		{Name: "Savannah Lions", Count: 6}, {Name: "Suntail Hawk", Count: 4}, {Name: "Serra Angel", Count: 2},
		{Name: "Plains", Count: 14},
	},
	{
		{Name: "Roads Go Ever, Ever On", Count: 7}, {Name: "Journey to Nowhere", Count: 4}, {Name: "Cloudshift", Count: 2},
		{Name: "Savannah Lions", Count: 6}, {Name: "Suntail Hawk", Count: 5}, {Name: "Serra Angel", Count: 2},
		{Name: "Plains", Count: 14},
	},
}}

func TestExwHandPickLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exwhand")
	if err != nil {
		t.Fatal(err)
	}
	tot := kexwTotals{ByCard: map[string][2]int{}}
	div, stalls := kexwPlayPair(t, reg, set, kexwHandPair, 1, 200, true, &tot)
	if div {
		return // kexwPlayPair reported the divergence
	}
	t.Logf("200 games, exw {Recorded:%d Reads:%d Matched:%d Rejected:%d}, by card %v",
		tot.Recorded, tot.Reads, tot.Matched, tot.Rejected, tot.ByCard)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	// the precondition the clean run depends on: the pick read the link and
	// found the Saga's Plains, and rejected cards another source exiled
	if r := tot.ByCard["Roads Go Ever, Ever On"]; r[0] < 20 || r[1] < 20 {
		t.Errorf("Roads Go Ever, Ever On: %d links recorded, %d reads matched; want at least 20 each", r[0], r[1])
	}
	if tot.Rejected == 0 {
		t.Error("no read rejected a card another source exiled")
	}
}
