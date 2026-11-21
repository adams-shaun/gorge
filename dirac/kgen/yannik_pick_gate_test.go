package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestYannikPickGateAndLKIScope: the battle-link pick's reflexive trigger
// reads the pick's own set. Its plain ConditionDefined$ Remembered gate does
// not OR in the persistent mark (the pick is the card's only remembered write
// and the chain ends in ClearRemembered$), and the pick-set arm of
// trigRemInit is handed only to a chain that holds the pick: Goblin Crash
// Pilot's RememberObjects$ RememberedLKI over a SacrificeAll keeps refusing
// its TriggerRemembered$CardPower read at the latent (the card-level remembered-reader refusal would
// otherwise name it, moving remspan.tsv).
func TestYannikPickGateAndLKIScope(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{allBlockers: true, byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	y := lower("Yannik, Scavenging Sentinel")
	if len(y.Refusals) != 0 {
		t.Fatalf("Yannik refusals = %v, want none", y.Refusals)
	}
	gates := 0
	for _, a := range y.Abilities {
		for _, in := range a.Prog {
			if in.Op != "SkipIf" {
				continue
			}
			gates++
			if strings.Contains(in.Cond.String(), "RemMark") {
				t.Errorf("Yannik's gate ORs the persistent mark: %s", in.Cond.String())
			}
		}
	}
	if gates == 0 {
		t.Fatal("Yannik lowers no SkipIf gate: the assertion above read nothing")
	}
	g := lower("Goblin Crash Pilot")
	if len(g.Refusals) == 0 {
		t.Fatal("Goblin Crash Pilot lowers: the Crewed trigger mode should still refuse it")
	}
	for _, r := range g.Refusals {
		if strings.Contains(r.Reason, "reads Remembered written by another ability") {
			t.Errorf("Goblin Crash Pilot's latent took the pick-set arm: %v", r)
		}
	}
}
