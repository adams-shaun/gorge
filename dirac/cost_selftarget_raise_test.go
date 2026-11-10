package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// selfTargetRaiseTable is an FDN Island/Plains matchup with the two cost
// statics a ValidTarget$ read can see on a card itself: the battlefield
// self-target RaiseCost (Icefall Regent's "spells your opponents cast that
// target CARDNAME cost {2} more", lowered with Zone left 0) and the card's own
// "this spell costs {2} less to cast if it targets ..." reduction (Zone
// ZoneCommand, Luminous Rebuke's shape).
func selfTargetRaiseTable(t *testing.T) (tbl *kop.Table, taxed, ownCost, island, plains CardID) {
	t.Helper()
	deckI := []cardset.Entry{{Name: "Island", Count: 20}}
	deckP := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deckI, deckP)
	if err != nil {
		t.Fatal(err)
	}
	var haveI, haveP bool
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Island":
			island, haveI = CardID(i), true
		case "Plains":
			plains, haveP = CardID(i), true
		}
	}
	if !haveI || !haveP {
		t.Fatalf("precondition: the pool holds the decks' lands (Island %d, Plains %d)", island, plains)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Flags: kop.FSelf})
	self := kop.FilterID(len(tbl.Filters) - 1)
	taxed = CardID(len(tbl.Cards))
	ownCost = taxed + 1
	cost := kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.U: 2}}
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Taxed Creature", Types: kop.TCreature, Power: 4, Toughness: 3, Cost: cost,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIncreaseCost,
				Filter2: self, A: 2, StFlags: kop.StTargetsFilter2 | kop.StRaiseOpp}}}},
		kop.CardDef{Name: "Test Own-Cost Reducer", Types: kop.TCreature, Power: 1, Toughness: 1, Cost: cost,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceCost,
				Filter2: self, A: 2, Zone: kop.ZoneCommand, StFlags: kop.StTargetsFilter2}}}})
	return tbl, taxed, ownCost, island, plains
}

// TestBattlefieldSelfTargetRaiseDoesNotDeclineOwnCast: a cost static that
// taxes spells targeting the permanent carrying it is a battlefield static,
// so it does not price that card's own cast from hand (CR 113.6, 601.2f).
// gorge's planner plans the cast, and the kernel must offer it from the
// untapped lands, not only from a floating pool.
func TestBattlefieldSelfTargetRaiseDoesNotDeclineOwnCast(t *testing.T) {
	tbl, taxed, ownCost, island, plains := selfTargetRaiseTable(t)

	st := tbl.Cards[taxed].Abilities[0].Static
	if st.Zone != kop.ZoneNone || st.StFlags&kop.StTargetsFilter2 == 0 || st.StFlags&kop.StRaiseOpp == 0 {
		t.Fatalf("precondition: the taxed creature carries the battlefield self-target raise, got zone %d flags %b", st.Zone, st.StFlags)
	}
	own := tbl.Cards[ownCost].Abilities[0].Static
	if own.Zone != kop.ZoneCommand || own.StFlags&kop.StTargetsFilter2 == 0 {
		t.Fatalf("precondition: the reducer carries the own-cast command-zone reduction, got zone %d flags %b", own.Zone, own.StFlags)
	}
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true, GorgeAutopaySurface: true})
	sc.lands(0, island, 5)
	o := sc.hand(taxed, 0)
	if sc.s.objs[o].zone != kop.ZoneHand {
		t.Fatalf("precondition: the creature is in hand, zone %d", sc.s.objs[o].zone)
	}
	sc.s.ctl = ctlPriority
	d := sc.s.Step()
	offered := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("the taxed creature's cast is not offered from five untapped Islands: options %v", d.Options[:d.N])
	}
	if sc.s.spellTgtDep(sc.k, 0, taxed, o) {
		t.Fatal("spellTgtDep says the taxed creature's own cast is target-priced")
	}
	if cardTgtDepCost(&tbl.Cards[taxed]) {
		t.Fatal("the battlefield self-target raise prices the card's own cast")
	}
	if !cardTgtDepCost(&tbl.Cards[ownCost]) {
		t.Fatal("the ZoneCommand own-cast reduction is still target-priced")
	}
}
