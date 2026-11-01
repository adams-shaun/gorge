package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrScen: Giada, two Plains and a Forest on p0's battlefield, Youthful
// Valkyrie (an Angel, {1}{W}) and Healer's Hawk (a Bird, {W}) in hand,
// and Giada tapped as a script / window gap: her Angel-only {W} floats
// beside the pool (poolR).
func restrScen(t *testing.T, cfg Config) (sc *scen, valk, hawk Obj, plains []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Giada, Font of Hope", Count: 1}, {Name: "Youthful Valkyrie", Count: 1},
		{Name: "Healer's Hawk", Count: 1}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 4}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	sc = newScenTable(t, tbl, cfg)
	giada := sc.perm(id["Giada, Font of Hope"], 0)
	plains = []Obj{sc.perm(id["Plains"], 0), sc.perm(id["Plains"], 0)}
	valk = sc.hand(id["Youthful Valkyrie"], 0)
	hawk = sc.hand(id["Healer's Hawk"], 0)
	var held [kop.NumManaKinds]uint8
	sl := sc.slot(giada)
	if !sc.s.windowRestr(sc.k, 0).has(sl) {
		t.Fatal("Giada is not a window restricted source")
	}
	sc.s.tapGap(sc.k, 0, sl, slotBit(sl), &held)
	if sc.s.pl[0].poolR&poolRCnt != 1 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("after Giada: poolR %#x pool %v, want one restricted {W} and an empty pool", sc.s.pl[0].poolR, sc.s.pl[0].pool)
	}
	return sc, valk, hawk, plains
}

func castOffered(d *Decision, o Obj) bool {
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == uint8(o) {
			return true
		}
	}
	return false
}

// TestRestrictedManaPaysAngel: an Angel spell cast while Giada's {W}
// floats spends it, before the ordinary {W} (gorge AvailableFor shows a
// matching batch to the payment; emitRestrictedManaSpend carves it
// first); a non-Angel cannot, and its pool alone does not pay it.
func TestRestrictedManaPaysAngel(t *testing.T) {
	// the kernel's own surface: the Valkyrie from one Plains and Giada's {W}
	sc, valk, _, plains := restrScen(t, Config{NoElide: true})
	s := sc.s
	sc.do(t, AkCast, uint8(valk), 0xff)
	s.Step() // the build pays
	if s.pl[0].poolR != 0 || s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("after the Valkyrie: poolR %#x pool %v, want both empty", s.pl[0].poolR, s.pl[0].pool)
	}
	if n := tappedN(sc, plains); n != 1 {
		t.Fatalf("%d Plains tapped, want 1 (Giada's {W} paid the pip)", n)
	}

	// gorge's autopay surface: a plan starts from the mana its payment may
	// spend (pay.PlanPool); the Angel's pool holds Giada's {W}, the Hawk's
	// does not
	sc, valk, hawk, _ := restrScen(t, Config{NoElide: true, GorgeAutopaySurface: true})
	s = sc.s
	s.pl[0].pool[kop.W] = 2
	d := s.Step()
	if !castOffered(&d, valk) || !castOffered(&d, hawk) {
		t.Fatalf("Valkyrie offered %v, Hawk %v; want both (the pool pays)", castOffered(&d, valk), castOffered(&d, hawk))
	}
	s.Apply(MakeAction(AkCast, uint8(valk), 0, 0))
	s.Step()
	if s.pl[0].poolR != 0 || s.pl[0].pool[kop.W] != 1 {
		t.Fatalf("after the Valkyrie: poolR %#x pool {W} %d, want the restricted {W} spent first, one ordinary {W} left", s.pl[0].poolR, s.pl[0].pool[kop.W])
	}

	// an inadmissible batch leaves the cast to its other sources: the Hawk's
	// {W} is a Plains; once they are tapped, only the Angel is offered
	sc, valk, hawk, plains = restrScen(t, Config{NoElide: true, GorgeAutopaySurface: true})
	s = sc.s
	s.pl[0].pool[kop.G] = 1
	d = s.Step()
	if !castOffered(&d, valk) || !castOffered(&d, hawk) {
		t.Fatalf("Valkyrie offered %v, Hawk %v; want both (the Hawk from a Plains)", castOffered(&d, valk), castOffered(&d, hawk))
	}
	if s.FeatPoolPays(0, hawk, 0) || !s.FeatPoolPays(0, valk, 0) {
		t.Fatal("FeatPoolPays: want the Valkyrie's pool to pay, the Hawk's not")
	}
	for _, o := range plains {
		s.perms[sc.slot(o)].flags |= pfTapped
	}
	d = s.Step()
	if !castOffered(&d, valk) || castOffered(&d, hawk) {
		t.Fatalf("Plains tapped: Valkyrie offered %v, Hawk %v; want the Angel only ({G} and the Angel-only {W})", castOffered(&d, valk), castOffered(&d, hawk))
	}
	s.Apply(MakeAction(AkCast, uint8(valk), 0, 0))
	s.Step()
	if s.pl[0].poolR != 0 || s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("after the Valkyrie: poolR %#x pool %v, want both empty", s.pl[0].poolR, s.pl[0].pool)
	}
}
