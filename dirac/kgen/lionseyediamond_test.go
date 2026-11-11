package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLionsEyeDiamondLowers: Lion's Eye Diamond's mana ability (discard the
// whole hand and sacrifice the source as its cost, instant timing) lowers
// with no refusal and builds a DiscardHand ability.
func TestLionsEyeDiamondLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Lion's Eye Diamond")
	if !ok || card == nil {
		t.Fatal("Lion's Eye Diamond not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	saw := false
	for i := range all[0].def.Abilities {
		if a := &all[0].def.Abilities[i]; kop.DiscardHandManaAb(a) {
			saw = true
			if a.ManaColors != kop.ColAny || a.ManaN != 3 {
				t.Errorf("mana %b x%d, want any one colour x3", a.ManaColors, a.ManaN)
			}
		}
	}
	if !saw {
		t.Error("Lion's Eye Diamond built with no discard-hand mana ability")
	}

	// A {T} beside the discard (Diamond Lion's shape) or no instant-only limit
	// stays refused: gorge would list either in its planner census.
	const body = "Name:Discard Hand Probe\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ %s | Produced$ Any | Amount$ 3%s\n"
	for _, c := range [][2]string{{"T Discard<0/Hand> Sac<1/CARDNAME>", " | InstantSpeed$ True"}, {"Sac<1/CARDNAME> Discard<0/Hand>", ""}} {
		src := strings.Replace(strings.Replace(body, "%s", c[0], 1), "%s", c[1], 1)
		if got := keclReason(discardHandSynth(t, src)); !strings.Contains(got, "discard-hand cost on a mana ability") {
			t.Errorf("%q%q: refusals %q, want the discard-hand mana refusal", c[0], c[1], got)
		}
	}
}
