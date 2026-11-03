package pickbot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// TestBuilderLeavesUnscriptedCardsOut: a pool card the corpus has no script
// for (a set drafted before its cards are scripted) never reaches the main
// deck, so the deck resolves; the corpus-backed cards still fill it.
func TestBuilderLeavesUnscriptedCardsOut(t *testing.T) {
	reg := corpus(t)
	var pool []limited.CardRef
	for i, n := range shortPairPoolNames[:23] {
		lookupCard(t, reg, n)
		pool = append(pool, ref("c"+string(rune('a'+i)), n, limited.Rarity('C')))
	}
	const ghost = "Card With No Script"
	if _, ok := reg.Lookup(ghost); ok {
		t.Fatalf("%q resolves", ghost)
	}
	pool = append(pool, ref("zz", ghost, limited.Rarity('M')))
	b := &pb.Best23{Lookup: lookupOf(reg)}
	d := b.Build("FDN", pool)
	for _, id := range d.Main {
		if id == "zz" {
			t.Fatalf("deck plays the unscripted card: %v", d.Main)
		}
	}
	if len(d.Main) == 0 {
		t.Fatal("empty main deck")
	}
}

// TestDrafterPassesUnscriptedCards: on a set no rating file covers, a
// mythic the corpus has no script for loses to a scripted common.
func TestDrafterPassesUnscriptedCards(t *testing.T) {
	reg := corpus(t)
	d := &pb.RatingColour{Lookup: lookupOf(reg)}
	v := pb.PickView{Set: "ZZZ", Round: 1, PickNo: 1, Pack: []limited.CardRef{
		ref("g", "Card With No Script", limited.Mythic),
		ref("h", "Healer's Hawk", limited.Common),
	}}
	if got := d.Pick(v, rand.New(rand.NewPCG(1, 2))); got != "h" {
		t.Fatalf("pick = %q, want the scripted card h", got)
	}
}
