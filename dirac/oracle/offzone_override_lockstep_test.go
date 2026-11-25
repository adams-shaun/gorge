package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The off-zone layer-4 override grants (dirac/kgen/lower_offzoneoverride.go,
// dirac/static_offzonechars.go): Maskwood Nexus (AffectedZone$ All,
// AddAllCreatureTypes$) and Rootpath Purifier (AffectedZone$ Battlefield,Library,
// AddType$ Basic). Each lowers as two statics: the plain battlefield override,
// which the layer walk reads (Goblin Warchief, Goblin King and Goblin Chieftain
// under Maskwood, Rootpath's basic lands), and the off-zone grant. Gorge's type
// and subtype filters read the layer-4 table, built from battlefield objects
// only, so a search, a cost reduction or a trigger reads a card in any other
// zone as printed, and so does the kernel: the off-zone half moves no decision
// (dirac.TestOffZoneCharsProject guards its projection). The pool is the
// off-battlefield statics fixture (offzonegrants).
var offZoneOverrideDecks = map[string][2][]cardset.Entry{
	"maskwood": {
		{
			{Name: "Maskwood Nexus", Count: 4}, {Name: "Goblin Matron", Count: 4}, {Name: "Goblin Warchief", Count: 4},
			{Name: "Goblin King", Count: 2}, {Name: "Goblin Chieftain", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 2},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Goblin Matron", Count: 4}, {Name: "Goblin Chieftain", Count: 4}, {Name: "Mogg Fanatic", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 6},
		},
	},
	"rootpath": {
		{
			{Name: "Rootpath Purifier", Count: 4}, {Name: "Rampant Growth", Count: 4}, {Name: "Kodama's Reach", Count: 3},
			{Name: "Evolving Wilds", Count: 3}, {Name: "Terramorphic Expanse", Count: 2},
			{Name: "Blood Crypt", Count: 3}, {Name: "Cinder Glade", Count: 3}, {Name: "Stomping Ground", Count: 2}, {Name: "Temple Garden", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Rampant Growth", Count: 4}, {Name: "Evolving Wilds", Count: 4}, {Name: "Blackcleave Cliffs", Count: 4},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 10},
		},
	},
}

func TestOffZoneOverrideLockstep(t *testing.T) {
	for _, name := range []string{"maskwood", "rootpath"} {
		t.Run(name, func(t *testing.T) {
			decks := offZoneOverrideDecks[name]
			_, set, gd := lockstepPool(t, offZoneUnearthPool, decks)
			if !SetDeckSupported(set, gd[0]) || !SetDeckSupported(set, gd[1]) {
				t.Fatal("the override decks are not playable in the offzonegrants pool")
			}
			lockstepRunPolicy(t, offZoneUnearthPool, decks, 64, true, nil, nil, castFirstPolicy)
		})
	}
}

// dropBattlefieldOverrides takes the battlefield half of the grants out of the
// table: a StatOverride whose record is every creature type or the basic
// supertype becomes a static of no kind.
func dropBattlefieldOverrides(tbl *kop.Table) {
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			st := &tbl.Cards[i].Abilities[j].Static
			if st.Kind == kop.StatOverride && st.Override != 0 {
				if o := &tbl.Overrides[st.Override-1]; o.AllCreatureTypes || o.AddTypes&kop.TBasic != 0 {
					st.Kind = 0
				}
			}
		}
	}
}

// TestOffZoneOverrideMutation proves the decks reach the grants: with the
// battlefield half dropped (Maskwood's Goblins stop pumping and hasting every
// creature, Rootpath's lands stop being basic), some game diverges from gorge.
func TestOffZoneOverrideMutation(t *testing.T) {
	total := 0
	for _, name := range []string{"maskwood", "rootpath"} {
		n := lockstepRunPolicy(t, offZoneUnearthPool, offZoneOverrideDecks[name], 64, false, dropBattlefieldOverrides, nil, castFirstPolicy)
		t.Logf("%s: %d games diverged without the grant", name, n)
		total += n
	}
	if total == 0 {
		t.Fatal("dropping the grants' battlefield half changed no game: the decks do not reach them")
	}
}

func TestFourSeatOffZoneOverrideLockstep(t *testing.T) {
	mask := pod4Deck(deckOf(4, "Maskwood Nexus"), deckOf(6, "Goblin Matron"), deckOf(3, "Goblin Recruiter"), deckOf(6, "Grizzly Bears"),
		deckOf(3, "Hill Giant"), deckOf(2, "Mogg Fanatic"), deckOf(9, "Mountain"), deckOf(7, "Forest"))
	root := pod4Deck(deckOf(4, "Rootpath Purifier"), deckOf(4, "Rampant Growth"), deckOf(3, "Kodama's Reach"), deckOf(3, "Evolving Wilds"),
		deckOf(2, "Terramorphic Expanse"), deckOf(3, "Blood Crypt"), deckOf(3, "Cinder Glade"), deckOf(2, "Stomping Ground"),
		deckOf(2, "Temple Garden"), deckOf(4, "Grizzly Bears"), deckOf(5, "Mountain"), deckOf(5, "Forest"))
	if n := pod4FixtureLockstep(t, "offzonegrants", [4][]string{mask[:40], root[:40], mask[:40], root[:40]}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
