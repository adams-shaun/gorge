package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

func builtWeftwalking(t *testing.T) kop.CardDef {
	t.Helper()
	lw := lowerNamedPool(t, []string{"Weftwalking"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var def kop.CardDef
	found := false
	for i, c := range all {
		if r := lowerRefusals(c); len(r) > 0 {
			t.Errorf("%s: refused: %v", c.Name, r)
		}
		if c.Name == "Weftwalking" {
			def, found = bs[i].def, true
		}
	}
	if !found {
		t.Fatal("Weftwalking not lowered")
	}
	return def
}

// TestWeftwalkingChangeZoneAllLowers: the Graveyard,Hand -> Library sweep
// builds as OpChangeZoneAllEx with MvGraveExile and the second zone Hand in B.
func TestWeftwalkingChangeZoneAllLowers(t *testing.T) {
	def := builtWeftwalking(t)
	n := 0
	for _, a := range def.Abilities {
		for _, in := range a.Prog {
			if in.Op != kop.OpChangeZoneAllEx {
				continue
			}
			n++
			if in.C>>3&kop.MvGraveExile == 0 || kop.Zone(in.C&7) != kop.ZoneLibrary {
				t.Errorf("C = %#x, want MvGraveExile into the library", in.C)
			}
			if in.B != kop.Const(int(kop.ZoneHand)) {
				t.Errorf("B = %+v, want Const(ZoneHand)", in.B)
			}
		}
	}
	if n != 1 {
		t.Fatalf("OpChangeZoneAllEx count = %d, want 1", n)
	}
}

// TestWeftwalkingStaticLowers: the first-spell free cast is a hand
// free-only StatMayPlayFrom, free, for the active player, under a ThisTurnCast cond.
func TestWeftwalkingStaticLowers(t *testing.T) {
	def := builtWeftwalking(t)
	n := 0
	for _, a := range def.Abilities {
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatMayPlayFrom {
			continue
		}
		n++
		st := a.Static
		if st.StFlags&kop.StFreeOnly == 0 {
			t.Errorf("flags = %#x, want StFreeOnly", st.StFlags)
		}
		if st.Zone != kop.ZoneHand {
			t.Errorf("zone = %v, want Hand", st.Zone)
		}
		want := kop.StWithoutManaCost | kop.StSeatActive
		if st.StFlags&want != want {
			t.Errorf("flags = %#x, want %#x set", st.StFlags, want)
		}
		if st.Cond2 == 0 {
			t.Error("no Cond2 (the first-spell gate)")
		}
	}
	if n != 1 {
		t.Fatalf("MayPlayFrom statics = %d, want 1", n)
	}
}

// TestFilterControlledByPlayerActive: the spaced spelling is the
// ActivePlayerCtrl word.
func TestFilterControlledByPlayerActive(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}
	a, err := lc.filter("Card.nonLand+ControlledBy Player.Active")
	if err != nil {
		t.Fatal(err)
	}
	b, err := lc.filter("Card.nonLand+ActivePlayerCtrl")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Clauses) != 1 || a.Clauses[0].CmpStat != cmpActiveCtrl {
		t.Fatalf("clauses = %+v, want CmpStat cmpActiveCtrl", a.Clauses)
	}
	if a.Clauses[0].NotTypes == 0 {
		t.Error("nonLand not kept")
	}
	if a.Clauses[0] != b.Clauses[0] {
		t.Errorf("%+v != %+v", a.Clauses[0], b.Clauses[0])
	}
}
