package cardset

import "testing"

// A card that chooses a card name needs the pool table to hold the choice in
// a byte; a pool whose nonland PoolIDs outgrow it plays the chooser inert
// (and says so), while a matchup, which numbers its cards compactly, still
// plays it with the name list.
func TestKaiWinnPoolNamesFit(t *testing.T) {
	s, err := RawByName("kaiwinn")
	if err != nil {
		t.Skip(err)
	}
	ds := s.(*dataSet)
	kw, ok := s.Lookup("Kai Winn, Faithless Leader")
	if !ok {
		t.Fatal("no Kai Winn in the fixture")
	}
	if s.Inert(kw) {
		t.Fatal("a small pool plays Kai Winn")
	}
	tbl, err := s.Pool()
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.NameCards) == 0 {
		t.Fatal("the small pool table lists no nameable cards")
	}
	for k := 1; k < len(tbl.NameCards); k++ {
		if tbl.NameCards[k-1] >= tbl.NameCards[k] {
			t.Fatalf("NameCards not ascending: %v", tbl.NameCards)
		}
	}

	// the same pool with one nonland card past the byte
	big := *ds.d
	big.PoolID = append([]uint16(nil), ds.d.PoolID...)
	gb, _ := s.Lookup("Grizzly Bears")
	big.PoolID[gb] = 300
	big.NumIDs = 301
	v := viewOf(&big, gated(false))
	if !v.Inert(kw) {
		t.Fatal("a pool whose nonland CardID is past the byte must play Kai Winn inert")
	}
	bt, err := v.Pool()
	if err != nil {
		t.Fatal(err)
	}
	if len(bt.NameCards) != 0 {
		t.Fatalf("an inert chooser must not ask for a name, NameCards = %v", bt.NameCards)
	}
	mt, _, _, err := v.Matchup([]Entry{{Name: "Kai Winn, Faithless Leader", Count: 1}, {Name: "Grizzly Bears", Count: 1}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mt.NameCards) == 0 {
		t.Fatal("a matchup holding the chooser must list its nameable cards")
	}
}
