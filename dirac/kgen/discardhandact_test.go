package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// discardHandSynth lowers a synthetic card script and returns its refusals.
func discardHandSynth(t *testing.T, src string) []refusal {
	t.Helper()
	card, diags := cards.ParseBytes("discardhand_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Discard Hand Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false).Refusals
}

// TestDiscardHandActCostLowers: Discard<N/Hand> in an activated ability's
// cost lowers to the DiscardHand table byte (the whole hand, in hand order,
// no choice and an empty hand payable); Discard<N/Random> stays an
// unless-cost selection method and refuses by name.
func TestDiscardHandActCostLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Reverberating Summons")
	if !ok || card == nil {
		t.Fatal("Reverberating Summons not in the corpus")
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
		a := &all[0].def.Abilities[i]
		if !a.Cost.DiscardHand {
			continue
		}
		saw = true
		if a.Cost.Flags&kop.CostDiscard != 0 || a.Cost.N != 0 {
			t.Errorf("DiscardHand ability flags %b N %d, want no CostDiscard and N 0", a.Cost.Flags, a.Cost.N)
		}
	}
	if !saw {
		t.Error("Reverberating Summons built with no Cost.DiscardHand ability")
	}

	const body = "Name:Discard Hand Probe\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ %s | LifeAmount$ 1\n"
	if got := keclReason(discardHandSynth(t, strings.Replace(body, "%s", "1 Discard<1/Random>", 1))); !strings.Contains(got, `discard cost "Random" outside an unless cost`) {
		t.Errorf("Discard<1/Random>: refusals %q, want the unless-cost reason", got)
	}
}
