package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestChooseTypeBasicLandEnters: Realmwright's as-enters "choose a basic land
// type" (gorge effChooseType over TypeChoiceLabels("Basic Land")) poses an ask
// of exactly the five basic land labels, not the creature-type vocabulary, and
// the answer is stored as the permanent's chosen matchup subtype, so the
// AddType$ ChosenType static gives every land that type.
func TestChooseTypeBasicLandEnters(t *testing.T) {
	set := cardsettest.Set(t, "choosebasicenter")
	deck := []cardset.Entry{{Name: "Realmwright", Count: 4}, {Name: "Island", Count: 18}, {Name: "Mountain", Count: 18}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{})
	sc.perm(id["Island"], 0)
	rw := sc.hand(id["Realmwright"], 0)
	sc.do(t, AkCast, TCode(rw), scAny)

	var ask *Decision
	for i := 0; i < 40 && ask == nil; i++ {
		d := sc.s.Step()
		switch {
		case d.Kind == DkChoose:
			ask = &d
		case d.Kind == DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			t.Fatalf("unexpected decision kind %d before the ask", d.Kind)
		}
	}
	if ask == nil || ask.Sub != ChGenType {
		t.Fatalf("Realmwright posed no type ask: %+v", ask)
	}
	if int(ask.N) != len(basicLandLabelH) {
		t.Fatalf("the as-enters basic land ask offers %d options, want the %d basic land types (a creature-type ask would offer %d)", ask.N, len(basicLandLabelH), len(tbl.TypeVocab))
	}
	offered := map[uint16]Action{}
	for i := 0; i < int(ask.N); i++ {
		a := ask.Options[i]
		offered[uint16(a.Target())<<8|uint16(a.Mode())] = a
	}
	for i, h := range basicLandLabelH {
		if _, ok := offered[h]; !ok {
			t.Errorf("the ask does not offer %s's label %#x", basicLandChoice[i], h)
		}
	}

	mtn := -1
	for i, n := range tbl.Subtypes {
		if n == "Mountain" {
			mtn = i
		}
	}
	if mtn < 0 {
		t.Fatalf("subtypes %v hold no Mountain", tbl.Subtypes)
	}
	sc.s.Apply(offered[basicLandLabelH[2]]) // Mountain
	sl := sc.slot(rw)
	if sl == NoSlot {
		for i := 0; i < 10 && sc.slot(rw) == NoSlot; i++ {
			d := sc.s.Step()
			if d.Kind != DkPriority {
				t.Fatalf("decision kind %d while Realmwright resolves", d.Kind)
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
		sl = sc.slot(rw)
	}
	if sl == NoSlot {
		t.Fatal("Realmwright never entered")
	}
	if got := sc.s.perms[sl].chosen; got != objWord(uint8(mtn)) {
		t.Errorf("Realmwright chose subtype %d, want Mountain (%d)", got, mtn)
	}
}
