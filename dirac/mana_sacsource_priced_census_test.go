package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorge's window census (rules castWindowCensusComplete) asks whether a source
// leaves a mana ability unpriced only for a source it priced nothing of. A
// tap-another or sacrifice-another source with a mana ability the window can
// take now, printed or granted, is priced and leaves the census complete; a
// spend-restricted one leaves it incomplete exactly when the payment is one its
// restriction admits (CR 106.6, 605.1a, 601.2g).

// sacPricedScen puts board under seat 0 (untapped, not summoning sick) in a
// hosted-cmdr table.
func sacPricedScen(t *testing.T, board ...string) (*scen, []Slot) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Forest", Count: 10}}
	for _, n := range board {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := cardsettest.Set(t, "hosted-cmdr").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	var slots []Slot
	for _, n := range board {
		sl := sc.slot(sc.perm(id[n], 0))
		sc.s.perms[sl].flags &^= pfTapped
		slots = append(slots, sl)
	}
	return sc, slots
}

// sacPricedSource asserts the precondition every case reads: sl is an
// activatable sacrifice-another or tap-another source of the search, so only
// the pricing rule can keep it out of the census gaps.
func sacPricedSource(t *testing.T, sc *scen, sl Slot) {
	t.Helper()
	if !sc.s.tapXSources(sc.k, 0, false).has(sl) {
		t.Fatalf("precondition: %s is no tap-another or sacrifice-another source", sc.k.t.Cards[sc.s.cardAt(sl)].Name)
	}
}

func sacPricedComplete(sc *scen) bool {
	return sc.s.windowCensusComplete(sc.k, &stackEntry{ctrl: 0})
}

// TestSacSourceWithPrintedManaLeavesCensusComplete: Phyrexian Tower's {C} is
// priced, so its "{T}, Sacrifice a creature" ability is no census gap, with no
// creature to sacrifice and with one.
func TestSacSourceWithPrintedManaLeavesCensusComplete(t *testing.T) {
	sc, sl := sacPricedScen(t, "Phyrexian Tower")
	sacPricedSource(t, sc, sl[0])
	if !sc.s.canTapForMana(sc.k, sl[0]) {
		t.Fatal("precondition: the Tower's printed {C} is unusable")
	}
	if !sacPricedComplete(sc) {
		t.Fatal("an untapped Tower with no creature leaves the census incomplete, gorge prices its {C}")
	}
	sc, sl = sacPricedScen(t, "Phyrexian Tower", "Llanowar Elves")
	sacPricedSource(t, sc, sl[0])
	if !sc.s.tapXCensusGap(sc.k, 0).has(sl[0]) {
		t.Fatal("precondition: the Tower with a creature has no sacrifice candidate")
	}
	if !sacPricedComplete(sc) {
		t.Fatal("an untapped Tower with a creature leaves the census incomplete, gorge prices its {C}")
	}
}

// TestSacSourceUnpricedLeavesCensusIncomplete: Kykar, Wind's Fury has only
// "Sacrifice a Spirit: Add {R}", unpriced with or without a Spirit. Paradise
// Mantle grants it a {T} ability the census prices; tapped, the grant is
// unusable and the sacrifice ability (no {T}) is a gap again.
func TestSacSourceUnpricedLeavesCensusIncomplete(t *testing.T) {
	sc, sl := sacPricedScen(t, "Kykar, Wind's Fury")
	sacPricedSource(t, sc, sl[0])
	if sacPricedComplete(sc) {
		t.Fatal("Kykar with no Spirit leaves the census complete, gorge cannot price its sacrifice cost")
	}
	sc, sl = sacPricedScen(t, "Kykar, Wind's Fury", "Paradise Mantle")
	kykar := sl[0]
	sc.s.attachTo(sc.k, sl[1], kykar)
	sacPricedSource(t, sc, kykar)
	if !sc.s.manaGranted(sc.k, kykar) || !sc.s.canTapForMana(sc.k, kykar) {
		t.Fatal("precondition: Paradise Mantle grants Kykar no usable mana ability")
	}
	if !sacPricedComplete(sc) {
		t.Fatal("Kykar with a granted mana ability leaves the census incomplete, gorge prices the grant")
	}
	sc.s.perms[kykar].flags |= pfTapped
	sc.s.touchSl(kykar)
	sacPricedSource(t, sc, kykar)
	if sacPricedComplete(sc) {
		t.Fatal("a tapped Kykar leaves the census complete, its sacrifice ability is still unpriced")
	}
}

// TestRestrictedSacSourceGapsOnlyAdmittedPayments: Master of Dark Rites'
// "Spend this mana only to cast Vampire, Cleric, and/or Demon spells" is a
// census gap for a Vampire spell and none for another payment.
func TestRestrictedSacSourceGapsOnlyAdmittedPayments(t *testing.T) {
	tbl, id := restrSacXTable(t, false, kop.Filter{})
	for _, c := range []struct {
		spell string
		want  bool
	}{{"Vampire Nighthawk", false}, {"Grizzly Bears", true}} {
		sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
		master := sc.slot(sc.perm(id["Master of Dark Rites"], 0))
		sacPricedSource(t, sc, master)
		o := sc.hand(id[c.spell], 0)
		s := sc.s
		b := s.bld()
		b.e = stackEntry{kind: seSpell, card: id[c.spell], ctrl: 0, obj: o, srcVer: s.objs[o].ver, tgt: noTargets}
		s.ctl = ctlBuild
		s.pushBuild(sc.k)
		d := s.payDescNow(NoObj)
		a := sc.k.tapXAbOf(s.cardAt(master))
		if a == nil || a.ManaRestrict == 0 || d.class != pdSpell || s.restrSrcAdmits(sc.k, 0, master, a, &d) == c.want {
			t.Fatalf("precondition: %s: ability %+v, class %d: want the restriction to admit it %v", c.spell, a, d.class, !c.want)
		}
		if got := s.windowCensusComplete(sc.k, &s.bld().e); got != c.want {
			t.Errorf("casting %s beside the Master: census complete %v, want %v", c.spell, got, c.want)
		}
	}
}
