package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// helperFloatScen is a board with Hydraulic Helper tapped for its {U}, which
// can't be spent to cast a nonartifact spell and so floats for an ability.
func helperFloatScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Hydraulic Helper", Count: 1}, {Name: "Island", Count: 16}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	set := cardsettest.Synth(t, "std-msh")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sl := sc.slot(sc.perm(id["Hydraulic Helper"], 0))
	var held [kop.NumManaKinds]uint8
	sc.s.tapGap(sc.k, 0, sl, slotBit(sl), &held)
	if sc.s.restrN(0) != 1 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("after the Helper: restricted %v pool %v, want one restricted {U} and an empty pool", sc.s.rb[0], sc.s.pl[0].pool)
	}
	return sc, id
}

// Floating restricted mana counts toward an activated ability's {X} when its
// restriction admits the ability (CR 106.6, CR 107.3). Stature entered this turn,
// so its power-up costs {U} less, {X}{U}: two Islands and the Helper's {U} reach
// X=2, where the Islands alone reach X=1.
func TestFloatingRestrictedManaBoundsAnActivationsX(t *testing.T) {
	sc, id := helperFloatScen(t, "Stature, Size Shifter")
	stature := sc.perm(id["Stature, Size Shifter"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	xs := sc.xChoices(t, stature, 0)
	if len(xs) == 0 || xs[len(xs)-1] != 2 {
		t.Fatalf("X choices %v, want a bound of 2", xs)
	}
}

// A filter's fee spends the floating restricted mana that may pay it before any
// ordinary unit, as the payment carves the admitted batches first: Surveillance
// Room's {1} takes the Helper's {U}, and the ordinary {R} stays in the pool.
func TestFilterFeeSpendsRestrictedManaFirst(t *testing.T) {
	sc, id := helperFloatScen(t, "Surveillance Room")
	s := sc.s
	room := sc.slot(sc.perm(id["Surveillance Room"], 0))
	if fee := s.filtFee(sc.k, room); fee != 1 {
		t.Fatalf("the Room's fee is %d, want 1", fee)
	}
	s.pl[0].pool[kop.R] = 1
	s.filtDo(sc.k, 0, filtStep{sl: room, gap: true, ans: 1 + uint8(kop.G)})
	if s.rbAny(0) {
		t.Fatalf("restricted %v, want the restricted {U} spent on the fee", s.rb[0])
	}
	if s.pl[0].pool[kop.R] != 1 || s.pl[0].pool[kop.G] != 1 {
		t.Fatalf("pool %v, want the {R} kept and the filter's {G} added", s.pl[0].pool)
	}
}
