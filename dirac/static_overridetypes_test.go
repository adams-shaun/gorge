package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStaticOverrideTypeFilter: an override static's filter type clause gates
// the types it adds, as it gates the P/T it sets (TestStaticOverrideLivePT's
// last subtest). Zoetic Glyph's "enchanted artifact is a Golem creature" is
// the lockstep shape: the host stops matching the filter, and the added
// Creature type must drop with the P/T, or the SBA reads a transformed land
// as a 0-toughness creature before the Aura has left (CR 704.5m, 613.1d).
func TestStaticOverrideTypeFilter(t *testing.T) {
	deck := []cardset.Entry{{Name: "Bonesplitter", Count: 1}, {Name: "Zoetic Glyph", Count: 1}, {Name: "Forest", Count: 18}}
	tbl, _, _, err := cardsettest.Set(t, "unattach").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var host, glyph, forest CardID
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Bonesplitter":
			host = CardID(c)
		case "Zoetic Glyph":
			glyph = CardID(c)
		case "Forest":
			forest = CardID(c)
		}
	}
	if tbl.Cards[host].Name != "Bonesplitter" || tbl.Cards[glyph].Name != "Zoetic Glyph" || tbl.Cards[forest].Name != "Forest" {
		t.Fatalf("test setup: card ids %d/%d/%d are not Bonesplitter/Zoetic Glyph/Forest", host, glyph, forest)
	}
	sc := newScenTableLib(t, tbl, forest, Config{NoElide: true})
	bones := sc.perm(host, 0)
	sl := sc.slot(bones)
	if !sc.onBF(bones) {
		t.Fatal("test setup: Bonesplitter is not on the battlefield")
	}
	if got := sc.s.typesGen(sc.k, sl); got&kop.TCreature != 0 || got&kop.TArtifact == 0 {
		t.Fatalf("test setup: Bonesplitter's types are %#x, want an artifact and no creature", got)
	}
	aura := sc.perm(glyph, 0)
	sc.s.attachTo(sc.k, sc.slot(aura), sl)
	sc.s.refreshTy(sc.k)
	if got := sc.s.typesGen(sc.k, sl); got&kop.TCreature == 0 {
		t.Fatalf("types %#x: the attached static's filter matches the artifact, so its added type is missing", got)
	}
	if p, tt := sc.s.pt(sc.k, sl); p != 5 || tt != 4 {
		t.Fatalf("under the Aura the host is %d/%d, want the override's 5/4", p, tt)
	}
	// Narrow the static's filter to a type the host lacks, as a host that
	// stopped being an artifact would: the clause must stop the add.
	for i := range tbl.Cards[glyph].Abilities {
		if a := &tbl.Cards[glyph].Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
			tbl.Filters[a.Static.Filter].Types = kop.TBasic
		}
	}
	sc.s.refreshTy(sc.k)
	if got := sc.s.typesGen(sc.k, sl); got&kop.TCreature != 0 {
		t.Fatalf("types %#x keep the added creature type on a host the filter no longer matches", got)
	}
	if p, tt := sc.s.pt(sc.k, sl); p != 0 || tt != 0 {
		t.Fatalf("P/T %d/%d on a host the filter no longer matches, want the printed 0/0", p, tt)
	}
}
