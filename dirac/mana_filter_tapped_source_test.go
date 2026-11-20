package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// hideoutBoard: p0 controls a Test Lotus Hideout ({T}: Add {C}; {1},{T}: Add
// one mana of any color) beside three Islands, with a {1}{W}{U} instant in
// hand. The {W} pip is payable only by the Hideout's paid ability (White Lotus
// Hideout's shape beside Islands, dirac/mana_filter_tapped_source.go), so
// every completing script activates it: the legal ones tap an Island for its
// {1} fee first, then the paid ability once; the illegal one the search
// reaches without the tapped-source guard is the free {T}: Add {C} first, then
// the paid {1},{T} ability from the same now-tapped source, its fee paid by a
// floated unit (CR 602.2b: the second {T} is unpayable).
func hideoutBoard(t *testing.T, order int) (*scen, Obj, [3]Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "hybridn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var island CardID = 0xffff
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == "Island" {
			island = CardID(c)
		}
	}
	if island == 0xffff {
		t.Fatal("the hybridn pool lacks Island")
	}
	cAb := kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: 1 << kop.C, ManaN: 1}
	fAb := kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}}, ManaColors: kop.ColAny, ManaN: 1}
	hideout := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Lotus Hideout", Types: kop.TLand, Abilities: []kop.Ability{cAb, fAb}})
	spell := CardID(len(tbl.Cards))
	var pips [kop.NumManaKinds]uint8
	pips[kop.W], pips[kop.U] = 1, 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Lotus Spell", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 1, Pips: pips}})
	sc := newScenTableLib(t, tbl, island, Config{NoElide: true})
	var ho Obj
	isles := [3]Obj{}
	if order == 0 { // the Hideout before the Islands in battlefield order
		ho = sc.perm(hideout, 0)
	} else {
		isles[0] = sc.perm(island, 0)
	}
	for i := range isles {
		if isles[i] == 0 {
			isles[i] = sc.perm(island, 0)
		}
	}
	if ho == 0 {
		ho = sc.perm(hideout, 0)
	}
	sp := sc.hand(spell, 0)
	return sc, ho, isles, sp
}

// hideoutActivations counts the found path's steps on the Hideout: a free step
// (gap, no ask) is its plain {T}, a paid one (gap, ans > 0) its {1},{T} ask.
func hideoutActivations(path []filtStep, sl Slot) (free, paid int) {
	for _, st := range path {
		if st.sl != sl || !st.gap || st.other.kind != otherNone {
			continue
		}
		if st.ans > 0 {
			paid++
		} else {
			free++
		}
	}
	return free, paid
}

// TestFiltPaidSkipsTappedFilter: the filter search's paid arm (visit's case
// filt) must not activate a {T}-bearing filter ability of a source the path
// has already tapped: its cost's {T} is unpayable (CR 602.2b), so the search
// prices the cast through a legal route instead (an Island's {U} paying the
// paid ability's {1} fee before the Hideout is tapped once), whatever the
// battlefield order.
func TestFiltPaidSkipsTappedFilter(t *testing.T) {
	for _, order := range []int{0, 1} {
		sc, ho, _, sp := hideoutBoard(t, order)
		s, k := sc.s, sc.k
		gaps := s.scriptFilt(k, 0)
		if !gaps.has(sc.slot(ho)) {
			t.Fatalf("order %d: precondition: the Hideout is no census filter gap (%v)", order, gaps)
		}
		if s.otherGaps(k, 0, MakeAction(AkPass, 0, 0, 0)).any() {
			t.Fatalf("order %d: precondition: the board has otherGaps", order)
		}
		d := s.Step()
		var cast Action
		found := false
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkCast && Obj(a.Source()) == sp {
				cast, found = a, true
			}
		}
		if !found {
			t.Fatalf("order %d: the {1}{W}{U} cast is not offered", order)
		}
		ok, path := filtSearchVisit(s, k, cast, s.playNeed(k, cast))
		if !ok {
			t.Fatalf("order %d: the search found no script for the cast", order)
		}
		free, paid := hideoutActivations(path, sc.slot(ho))
		if free+paid > 1 {
			t.Fatalf("order %d: the found path activates the Hideout free %d and paid %d times (its {1},{T} ability from a source the path already tapped): %v",
				order, free, paid, path)
		}
	}
}
