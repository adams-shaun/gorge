package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 305.6: Kyoshi Village ({T}: Add {G} or {W}, Combo G W) under a granted
// Island answers its printed ability first, {G}, not the lowest colour {W};
// an owed {W} still wins the window.
func TestLandGrantComboKeepsTokenOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "The Legend of Kyoshi", Count: 4}, {Name: "Kyoshi Village", Count: 8}, {Name: "Mountain", Count: 20}}
	set := cardsettest.Synth(t, "std-tla")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-tla table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{})
	g := sc.k.gen
	c := id["Kyoshi Village"]
	if !g.blgOn || g.blgIdx[c] < 0 {
		t.Fatal("no granted-type views for Kyoshi Village")
	}
	island := kop.ColorMask(1) << kop.U
	v := &g.blgPC[int(g.blgIdx[c])*32+int(island)]
	if v.blgFirst != uint8(kop.G)+1 {
		t.Fatalf("blgFirst %d, want %d (G first by the Combo's token order)", v.blgFirst, kop.G+1)
	}
	if v.manaCol&(1<<kop.W) == 0 || v.manaCol&(1<<kop.G) == 0 || v.manaCol&island == 0 {
		t.Fatalf("view manaCol %b lacks G, W or U", v.manaCol)
	}
	sl := Slot(sc.perm(c, 0))
	both := cost(0, kop.W, kop.G)
	if col := sc.s.windowCol(sc.k, 0, &both, sl, v, false, false); col != kop.G {
		t.Errorf("{W}{G} owed: window answers %d, want G (token order)", col)
	}
	onlyW := cost(0, kop.W)
	if col := sc.s.windowCol(sc.k, 0, &onlyW, sl, v, false, false); col != kop.W {
		t.Errorf("{W} owed: window answers %d, want W", col)
	}
	if col := sc.s.windowCol(sc.k, 0, &onlyW, sl, v, false, true); col != kop.G {
		t.Errorf("nothing owed: window answers %d, want G", col)
	}
}
