package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfReductionFromGraveyard: Tolarian Terror's "costs {1} less for
// each instant and sorcery card in your graveyard" applies at the offer of
// its owner's cast from the graveyard through Muldrotha (CR 601.2f: the
// cost is determined as the spell is on the stack), so five instants make
// it {1}{U} and two Islands cast it (pair 4094). Before, the filter's stack
// zone failed for the card in the graveyard and the cast cost {6}{U}.
func TestSelfReductionFromGraveyard(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Tolarian Terror", Count: 1}, {Name: "Muldrotha, the Gravetide", Count: 1}, {Name: "Refute", Count: 5}, {Name: "Island", Count: 8}})
	sc.perm(id["Muldrotha, the Gravetide"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	for i := 0; i < 5; i++ {
		sc.s.place(sc.obj(id["Refute"], 0), kop.ZoneGraveyard, false)
	}
	tt := sc.obj(id["Tolarian Terror"], 0)
	sc.s.place(tt, kop.ZoneGraveyard, false)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(tt)) {
			return
		}
	}
	t.Fatalf("Tolarian Terror's {1}{U} graveyard cast not offered: %v", d.Options[:d.N])
}

// TestStashCastRiderPipIsGeneric: gorge's planner funds a Tinybones stash
// cast's pips as generic (pay.PlanManaUnderRider), so Healer's Hawk's {W} is {1}
// to the plan: the Swamp pays it whatever type Secluded Courtyard names, the
// Courtyard pays it alone with the Swamp tapped, and it plans the Swamp first.
// The rider relaxes what mana pays, not how much: no source, no offer.
func TestStashCastRiderPipIsGeneric(t *testing.T) {
	deck := []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Healer's Hawk", Count: 1}, {Name: "Secluded Courtyard", Count: 1}, {Name: "Swamp", Count: 8}}
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
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeUnpriced: true})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	swamp := sc.perm(id["Swamp"], 0)
	yard := sc.perm(id["Secluded Courtyard"], 0)
	hk := sc.obj(id["Healer's Hawk"], 1)
	sc.s.place(hk, kop.ZoneExile, false)
	sc.s.stashSet(sc.k, hk)
	d := sc.s.Step()
	for i := 0; i < 5 && d.Kind != DkPriority; i++ {
		sc.s.Apply(d.Options[0]) // the Courtyard's creature type, then set: Bird
		d = sc.s.Step()
	}
	bird := -1
	for i, n := range tbl.Subtypes {
		if n == "Bird" {
			bird = i
		}
	}
	if bird < 0 {
		t.Skip("no Bird subtype in the table")
	}
	cs := -1
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		if sc.k.t.Cards[sc.s.cardAt(m.first())].Name == "Secluded Courtyard" {
			cs = int(m.first())
		}
	}
	if cs < 0 {
		t.Fatal("no Secluded Courtyard")
	}
	hawkOffered := func() bool {
		d = sc.s.Step()
		return castOffered(&d, hk)
	}
	for _, ch := range []int{bird, -1} {
		if ch < 0 {
			for i, n := range tbl.Subtypes {
				if n != "Bird" {
					ch = i
					break
				}
			}
		}
		sc.s.perms[cs].chosen = objWord(uint8(ch))
		if !hawkOffered() {
			t.Fatalf("type %d chosen (Bird %d): the stashed Healer's Hawk not offered from the Swamp: %v", ch, bird, d.Options[:d.N])
		}
		sc.s.perms[sc.slot(swamp)].flags |= pfTapped
		if !hawkOffered() {
			t.Fatalf("type %d chosen (Bird %d): the stashed Healer's Hawk not offered from the Courtyard: %v", ch, bird, d.Options[:d.N])
		}
		sc.s.perms[sc.slot(yard)].flags |= pfTapped
		if hawkOffered() {
			t.Fatalf("type %d chosen (Bird %d): the stashed Healer's Hawk offered with no source: %v", ch, bird, d.Options[:d.N])
		}
		sc.s.perms[sc.slot(swamp)].flags &^= pfTapped
		sc.s.perms[sc.slot(yard)].flags &^= pfTapped
	}
	sc.do(t, AkCast, TCode(hk), scAny)
	sc.s.Step()
	if sc.s.objs[hk].zone != kop.ZoneStack || !sc.tapped(swamp) || sc.tapped(yard) {
		t.Fatalf("zone %d, Swamp tapped %v, Courtyard tapped %v; want the stack and the Swamp alone", sc.s.objs[hk].zone, sc.tapped(swamp), sc.tapped(yard))
	}
}

// TestGrantedFlashbackBesidesPrinted: a card with a printed flashback
// (Inspiration from Beyond's {5}{U}{U}) granted flashback by Sphinx of
// Forgotten Lore has a second instance costing its mana cost {2}{U}; gorge
// offers it as its own cast (rules extraFlashbackCosts, CR 702.34a), so
// three Islands cast it (pair 1841). Before, the kernel priced only the
// printed instance.
func TestGrantedFlashbackBesidesPrinted(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Inspiration from Beyond", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Island", Count: 8}})
	for i := 0; i < 3; i++ {
		sc.perm(id["Island"], 0)
	}
	ib := sc.obj(id["Inspiration from Beyond"], 0)
	sc.s.place(ib, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, ib)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(ib)) && a.Mode()&2 != 0 {
			sc.s.Apply(a)
			sc.s.Step()
			if z := sc.s.objs[ib].zone; z != kop.ZoneStack {
				t.Fatalf("Inspiration from Beyond zone %d after the cast, want the stack", z)
			}
			return
		}
	}
	t.Fatalf("the granted {2}{U} flashback is not offered: %v", d.Options[:d.N])
}

// TestGrantedFlashbackInstanceNotPlanned: the granted instance of a
// flashback beside a printed one is a cast option carrying its own cost
// (Option.Cost), which gorge's payment-action builder never plans
// (planCastFlashbackOption needs Cost ""), so it is not one of the net
// list's synthetic casts: Inspiration from Beyond's {2}{U} instance is
// encoded as the list's last entry (pair 1841 -features). Before,
// FeatPlannedCast priced the printed {5}{U}{U} and called it planned.
func TestGrantedFlashbackInstanceNotPlanned(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Inspiration from Beyond", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Island", Count: 8}})
	for i := 0; i < 3; i++ {
		sc.perm(id["Island"], 0)
	}
	ib := sc.obj(id["Inspiration from Beyond"], 0)
	sc.s.place(ib, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, ib)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(ib)) && a.Mode()&2 != 0 {
			if sc.s.FeatPlannedCast(0, ib, a.Mode()) {
				t.Fatal("the granted {2}{U} flashback instance is reported planned")
			}
			return
		}
	}
	t.Fatalf("the granted {2}{U} flashback is not offered: %v", d.Options[:d.N])
}

// TestGrantedFlashbackAmbiguous: Think Twice's printed {2}{U} and granted
// {1}{U} flashbacks are both in gorge's potential walk with a Dismal
// Backwater and an Island (its potential pool counts the dual as one blue
// and one black), so the two casts are "ambiguous" and the oracle surface
// offers neither (pair 7295).
func TestGrantedFlashbackAmbiguous(t *testing.T) {
	deck := []cardset.Entry{{Name: "Think Twice", Count: 1}, {Name: "Dismal Backwater", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Island", Count: 8}}
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
	for _, surface := range []bool{true, false} {
		sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: surface})
		bw := sc.perm(id["Dismal Backwater"], 0)
		sc.s.perms[sc.slot(bw)].flags &^= pfTapped
		sc.perm(id["Island"], 0)
		tt := sc.obj(id["Think Twice"], 0)
		sc.s.place(tt, kop.ZoneGraveyard, false)
		sc.s.grantFB(sc.k, tt)
		d := sc.s.Step()
		for i := 0; i < 5 && d.Kind != DkPriority; i++ {
			sc.s.Apply(d.Options[0])
			d = sc.s.Step()
		}
		got := false
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkCast && a.Source() == TCode(uint8(tt)) {
				got = true
			}
		}
		if got == surface {
			t.Errorf("surface %v: Think Twice's flashback offered %v", surface, got)
		}
	}
}

// TestSelfReductionStashCaster: Tolarian Terror cast by p0 from p1's
// stash (Tinybones) counts the CASTER's instants and sorceries (CR 601.2a,
// 601.2f: "your graveyard" is the caster's while the cost is determined):
// five in p0's graveyard make it {1}{U}, two Islands cast it (pairs 8835
// 9227 9239). Before, the kernel applied no reduction to
// another player's card.
func TestSelfReductionStashCaster(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Tolarian Terror", Count: 1}, {Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Refute", Count: 5}, {Name: "Island", Count: 8}})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	for i := 0; i < 5; i++ {
		sc.s.place(sc.obj(id["Refute"], 0), kop.ZoneGraveyard, false)
	}
	tt := sc.obj(id["Tolarian Terror"], 1)
	sc.s.place(tt, kop.ZoneExile, false)
	sc.s.stashSet(sc.k, tt)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(tt)) {
			return
		}
	}
	t.Fatalf("p1's stashed Tolarian Terror is not offered at {1}{U}: %v", d.Options[:d.N])
}

// TestPrintedFlashbackPlanned: with three untapped sources Think Twice's
// printed {2}{U} flashback is the planner's planned cast and is offered
// under the oracle surface although its granted {1}{U} instance is listed
// too (pair 90106; the 100k census caught f023201 withholding it).
func TestPrintedFlashbackPlanned(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Think Twice", Count: 1}, {Name: "Island", Count: 8}})
	sc.s.cfg |= cfgGorgeAutopay
	for i := 0; i < 3; i++ {
		sc.perm(id["Island"], 0)
	}
	tt := sc.obj(id["Think Twice"], 0)
	sc.s.place(tt, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, tt)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(tt)) {
			return
		}
	}
	t.Fatalf("Think Twice's printed flashback not offered: %v", d.Options[:d.N])
}

// TestStashRebukeCheapestTarget: a Luminous Rebuke cast from Tinybones' stash is
// priced at its cheapest legal target (CR 601.2f): W U B floating and a tapped
// creature to target pay {1}{W} under the rider.
func TestStashRebukeCheapestTarget(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Luminous Rebuke", Count: 1}, {Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Plains", Count: 8}})
	sc.s.cfg |= cfgGorgeAutopay
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	r := sc.perm(id["Burglar Rat"], 1)
	sc.s.perms[sc.slot(r)].flags |= pfTapped
	sc.s.pl[0].pool[kop.W], sc.s.pl[0].pool[kop.U], sc.s.pl[0].pool[kop.B] = 1, 1, 1
	lr := sc.obj(id["Luminous Rebuke"], 1)
	sc.s.place(lr, kop.ZoneExile, false)
	sc.s.stashSet(sc.k, lr)
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(lr)) {
			return
		}
	}
	t.Fatalf("the stashed Luminous Rebuke is not offered from a three-mana pool: %v", d.Options[:d.N])
}

// TestStashCastRiderPlansEverySource: under the rider Dreadwing Scavenger's
// {1}{U}{B} is {3} to gorge's planner, so two Swamps and Secluded Courtyard fund
// it and the cast taps all three, the Courtyard for its {C} though no source
// makes {U}. Two of the three are one short: the rider relaxes colour, not count.
func TestStashCastRiderPlansEverySource(t *testing.T) {
	deck := []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Dreadwing Scavenger", Count: 1}, {Name: "Secluded Courtyard", Count: 1}, {Name: "Swamp", Count: 8}}
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
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeUnpriced: true})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	lands := []Obj{sc.perm(id["Swamp"], 0), sc.perm(id["Swamp"], 0), sc.perm(id["Secluded Courtyard"], 0)}
	dw := sc.obj(id["Dreadwing Scavenger"], 1)
	sc.s.place(dw, kop.ZoneExile, false)
	sc.s.stashSet(sc.k, dw)
	d := sc.s.Step()
	for i := 0; i < 5 && d.Kind != DkPriority; i++ {
		sc.s.Apply(d.Options[0]) // the Courtyard's creature type
		d = sc.s.Step()
	}
	if d.Kind != DkPriority {
		t.Fatalf("no priority decision: %v", d.Kind)
	}
	if !castOffered(&d, dw) {
		t.Fatalf("the stashed Dreadwing Scavenger is not offered from three sources: %v", d.Options[:d.N])
	}
	sc.s.perms[sc.slot(lands[0])].flags |= pfTapped
	if d = sc.s.Step(); castOffered(&d, dw) {
		t.Fatalf("the stashed Dreadwing Scavenger is offered from two sources: %v", d.Options[:d.N])
	}
	sc.s.perms[sc.slot(lands[0])].flags &^= pfTapped
	sc.do(t, AkCast, TCode(dw), scAny)
	sc.s.Step()
	if sc.s.objs[dw].zone != kop.ZoneStack || tappedN(sc, lands) != 3 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("zone %d, %d lands tapped, pool %v; want the stack, all three and an empty pool", sc.s.objs[dw].zone, tappedN(sc, lands), sc.s.pl[0].pool)
	}
}
