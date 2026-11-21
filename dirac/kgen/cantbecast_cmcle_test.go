package kgen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBriselaCantBeCastCmcLE3: Brisela, Voice of Nightmares' "your opponents
// can't cast spells with mana value 3 or less" (S:Mode$ CantBeCast |
// ValidCard$ Card.cmcLE3) lowers. The carrier is the meld result face of
// Gisela, the Broken Blade: Gisela and Bruna lower on the meld layout, and the
// face's static lowers with a constant mana-value bound (CR 202.3e; the
// kernel's X look-ahead in cast_cantself.go reads it).
func TestBriselaCantBeCastCmcLE3(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Gisela, the Broken Blade\nBruna, the Fading Light\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range append(lw.cards, lw.tokens...) {
		byName[c.Name] = c
	}

	for _, name := range []string{"Gisela, the Broken Blade", "Bruna, the Fading Light"} {
		c := byName[name]
		if c == nil {
			t.Fatalf("%s: not lowered", name)
		}
		if len(c.Refusals) != 0 {
			t.Fatalf("%s: refusals %+v, want none", name, c.Refusals)
		}
	}

	b := byName["face:Gisela, the Broken Blade#1"]
	if b == nil {
		t.Fatal("Brisela's face entry: not lowered")
	}
	if len(b.Refusals) != 0 {
		t.Fatalf("Brisela's face entry: refusals %+v, want none", b.Refusals)
	}
	var got *lStatic
	for i := range b.Statics {
		if b.Statics[i].Kind == "CantBeCast" {
			got = &b.Statics[i]
		}
	}
	if got == nil {
		t.Fatalf("Brisela's face entry: no CantBeCast static in %+v", b.Statics)
	}
	if got.Flags == nil || len(got.Flags) != 1 || got.Flags[0] != "ActivatorOpp" {
		t.Errorf("Brisela's CantBeCast flags = %v, want [ActivatorOpp]", got.Flags)
	}
	if got.F == nil || len(got.F.Clauses) != 1 {
		t.Fatalf("Brisela's CantBeCast filter = %+v, want one mana-value clause", got.F)
	}
	cl := got.F.Clauses[0]
	if cl.CmpStat != cmpCMC || cl.CmpOp != opLE || cl.CmpN != 3 {
		t.Errorf("Brisela's CantBeCast clause = %+v, want cmc <= 3", cl)
	}
}
