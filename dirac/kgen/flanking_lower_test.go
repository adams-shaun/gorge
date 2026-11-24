package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// flankingCarriers print one bare K:Flanking (CR 702.25a) and nothing else
// kgen refuses; each lowers to CardDef.Flanking plus gorge's expansion trigger
// (Mode$ AttackerBlockedByCreature, ValidBlocker$ Creature.withoutFlanking).
var flankingCarriers = []string{
	"Benalish Cavalry", "Bogardan Lancer", "Fallen Askari", "Femeref Knight",
	"Knight of the Mists", "Mtenda Herder", "Pentarch Paladin",
	"Shadow Rider", "Sidar Jabari", "Skulking Knight", "Suq'Ata Lancer", "Zhalfirin Commander",
	"Zhalfirin Knight",
}

func lowerNamedCard(t *testing.T, name string) *lCard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerCard(p, card.Faces[0].Name, card, false)
}

func TestFlankingCarriersLower(t *testing.T) {
	for _, name := range flankingCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if !c.Flanking {
			t.Errorf("%s: CardDef.Flanking not set", name)
		}
	}
}

// TestWithoutFlankingWord: Creature.withoutFlanking lowers to PredWord7 /
// W7WithoutFlanking; the positive word lowers to FwWithKeyword over
// CardDef.Flanking (T13b-2), and a granted Flanking (Cavalry Master) stays
// refused by name.
func TestWithoutFlankingWord(t *testing.T) {
	cb := &cardBuilder{pt: &poolTables{missing: map[string]bool{}}}
	f, err := parseFilter("Creature.withoutFlanking", func(string) bool { return false }, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
	if err != nil {
		t.Fatal(err)
	}
	if kf.Pred != kop.PredWord7 || kf.PredN != kop.W7WithoutFlanking {
		t.Fatalf("Pred %d PredN %d, want PredWord7 / W7WithoutFlanking", kf.Pred, kf.PredN)
	}
	wf, err := parseFilter("Creature.withFlanking", func(string) bool { return false }, func(string) bool { return false })
	if err != nil {
		t.Fatalf("withFlanking: %v", err)
	}
	if _, err := cb.clause(wf.Clauses[0], kop.ZoneBattlefield); err != nil {
		t.Errorf("withFlanking clause: %v", err)
	}
	c := lowerNamedCard(t, "Cavalry Master")
	if len(c.Refusals) == 0 {
		t.Errorf("Cavalry Master (a granted flanking) lowers; want a refusal")
	}
}
