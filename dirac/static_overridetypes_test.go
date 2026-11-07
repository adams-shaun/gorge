package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStaticOverrideTypeFilter: an override static's filter type clause gates
// the types it adds, as it gates the P/T it sets (TestStaticOverrideLivePT's
// last subtest). Treasure Map below Zoetic Glyph is the lockstep case: the
// Aura's "enchanted artifact" override adds the Creature type and 5/4, and
// the host transforming into Treasure Cove stops being an artifact. The
// clause must drop the added type at once; otherwise the SBA reads the land
// as a 0-toughness creature before the Aura has left (CR 704.5m, 613.1d).
func TestStaticOverrideTypeFilter(t *testing.T) {
	deck := []cardset.Entry{{Name: "Treasure Map", Count: 1}, {Name: "Zoetic Glyph", Count: 1}, {Name: "Forest", Count: 18}}
	tbl, _, _, err := cardsettest.Synth(t, "std-lci").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var mapID, glyphID, forest CardID
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Treasure Map":
			mapID = CardID(c)
		case "Zoetic Glyph":
			glyphID = CardID(c)
		case "Forest":
			forest = CardID(c)
		}
	}
	if tbl.Cards[mapID].Name != "Treasure Map" || tbl.Cards[glyphID].Name != "Zoetic Glyph" || tbl.Cards[forest].Name != "Forest" {
		t.Fatalf("test setup: card ids %d/%d/%d are not Treasure Map/Zoetic Glyph/Forest", mapID, glyphID, forest)
	}
	sc := newScenTableLib(t, tbl, forest, Config{NoElide: true})
	tm := sc.perm(mapID, 0)
	sl := sc.slot(tm)
	if !sc.onBF(tm) {
		t.Fatal("test setup: Treasure Map is not on the battlefield")
	}
	if sc.s.typesGen(sc.k, sl)&kop.TCreature != 0 {
		t.Fatal("test setup: Treasure Map is a creature before the Aura")
	}
	glyph := sc.perm(glyphID, 0)
	sc.s.attachTo(sc.k, sc.slot(glyph), sl)
	sc.s.refreshTy(sc.k)
	if got := sc.s.typesGen(sc.k, sl); got&kop.TCreature == 0 || got&kop.TArtifact == 0 {
		t.Fatalf("types %#x under Zoetic Glyph, want an artifact creature (the Aura's override)", got)
	}
	// The transform flips the same object to Treasure Cove.
	sc.s.transformObj(sc.k, tm)
	if face := tbl.Cards[sc.s.objs[tm].card]; face.Face != 1 || face.Types&kop.TLand == 0 {
		t.Fatalf("test setup: the object's face is %q (face %d, types %#x), want the land back face", face.Name, face.Face, face.Types)
	}
	if got := sc.s.typesGen(sc.k, sl); got&kop.TCreature != 0 {
		t.Fatalf("types %#x keep the Aura's added creature type on a host its filter no longer matches", got)
	}
	if got := sc.s.typesGen(sc.k, sl); got&kop.TLand == 0 {
		t.Fatalf("types %#x: the transformed land lost its own type", got)
	}
}
