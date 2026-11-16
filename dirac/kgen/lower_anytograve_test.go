package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "Whenever a card is put into a graveyard from anywhere" (ChangesZone
// Origin$ Any, Destination$ Graveyard, a battlefield listener) lowers as two
// kernel triggers with one body: a ToGraveyard line over the origins but the
// battlefield and a Dies line (lower_anytograve.go). The cards that listen from
// the graveyard they land in (Dread: Card.Self) or from another zone (Nihilith:
// exile) still refuse.
func TestAnyToGraveyardTriggersLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg,
			typeWords: map[string]bool{"Desert": true}} // the corpus vocabulary Yuma's Desert filter reads
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	want := int16(0)
	for _, z := range []kop.Zone{kop.ZoneHand, kop.ZoneGraveyard, kop.ZoneExile, kop.ZoneStack, kop.ZoneLibrary, kop.ZoneCommand} {
		want |= 1 << uint(z)
	}
	for _, name := range []string{
		"Compost", "Countryside Crusher", "Measure of Wickedness", "Planar Void", "Profane Memento",
		"Quest for Ancient Secrets", "Skola Grovedancer", "Slogurk, the Overslime", "Soulcipher Board",
		"The Haunt of Hightower", "Vulturous Zombie", "Yuma, Proud Protector",
	} {
		c := lower(name)
		for _, rf := range c.Refusals {
			t.Errorf("%s refused: %s", name, rf.Reason)
		}
		toGrave, dies := 0, 0
		for _, a := range c.Abilities {
			if a.Trigger == nil {
				continue
			}
			switch a.Trigger.Event {
			case "ToGraveyard":
				toGrave++
				if a.Trigger.N != want {
					t.Errorf("%s: ToGraveyard mask %d, want %d (every origin but the battlefield)", name, a.Trigger.N, want)
				}
			case "Dies":
				dies++
			}
		}
		// the one printed Any->Graveyard line is exactly one of each
		// (Slogurk also has a self leave line, which is neither)
		if toGrave != 1 || dies != 1 {
			t.Errorf("%s lowered %d ToGraveyard and %d Dies lines, want 1 and 1", name, toGrave, dies)
		}
	}
	for _, name := range []string{"Dread", "Serra Avatar", "Worldspine Wurm", "Duskmantle Guildmage", "Nihilith"} {
		if c := lower(name); len(c.Refusals) == 0 {
			t.Errorf("%s lowers: its Any->Graveyard line listens from another zone and must still refuse", name)
		}
	}
}
