package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// staticPreventCards are the corpus cards whose only blocker was a printed
// static DamageDone prevention: a full one (Prevent$ True) or a prevent-N
// shield (DB$ ReplaceDamage | Amount$ N).
var staticPreventCards = []string{
	"Artifact Ward", "Battletide Alchemist", "Blessed Sanctuary", "Bubble Matrix", "Callous Giant",
	"Daunting Defender", "Defang", "Djeru, With Eyes Open", "Dolmen Gate", "Drogskol Reinforcements",
	"Emmara Tandris", "Energy Storm", "Goblin Furrier", "Guardian Seraph", "Hyperion, Supreme Hero",
	"Indentured Oaf", "Inviolability", "Light of Sanction", "Losheel, Clockwork Scholar", "Magebane Armor",
	"Mark of Asylum", "Multiclass Baldric", "Muzzle", "Orbs of Warding", "Personal Sanctuary",
	"Plated Pegasus", "Prismatic Ward", "Protection of the Hekma", "Rescue Retriever", "Rock Hydra",
	"Shield of the Avatar", "Shield of the Realm", "Sphere of Duty", "Sphere of Grace", "Sphere of Law",
	"Sphere of Purity", "Sphere of Reason", "Sphere of Truth", "Spirit of Resistance", "Statecraft",
	"Temple Altisaur", "The Wanderer", "Thunderstaff", "Urza's Armor", "Well-Laid Plans",
}

// TestStaticDamagePreventionLowers: every printed static damage prevention of
// the corpus lowers with no refusal, and each carries a prevention replacement
// of one of the two static kinds (a full prevention or a prevent-N shield).
func TestStaticDamagePreventionLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range staticPreventCards {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
			continue
		}
		found := false
		for _, r := range lc.Repls {
			if r.Kind == "PreventDamageFilt" || r.Kind == "PreventDamageN" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s lowers without a static prevention replacement (Repls %v)", name, lc.Repls)
		}
	}
}
