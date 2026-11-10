package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterXScen: a hand-built table with a permanent whose activated ability
// costs {X} plus SubCounter<X/P1P1> (the shared-X shape), a SubCounter<X>
// ability with no mana, and a planeswalker-style [-X]; p0 holds `lands`
// Plains and the {X} permanent carries `ctrs` +1/+1 counters.
func counterXScen(t *testing.T, lands, ctrs int) (sc *scen, tbl *kop.Table, sentry, plain, walker CardID, src Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	body := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.ValueRef{K: kop.VX}}}
	add := func(name string, c kop.Cost) CardID {
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: name, Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Cost: c, Prog: body}}})
		return id
	}
	sentry = add("Test Counter Sentry", kop.Cost{Mana: kop.ManaCost{X: 1}, Flags: kop.CostLoyaltyX, Ctr: kop.CtrP1P1})
	plain = add("Test Counter Plain", kop.Cost{Flags: kop.CostLoyaltyX, Ctr: kop.CtrP1P1})
	walker = add("Test Counter Walker", kop.Cost{Flags: kop.CostLoyalty | kop.CostLoyaltyX, Ctr: kop.CtrLoyalty})
	sc = newScenTableLib(t, tbl, idOf(tbl, da, "Plains"), Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyLoyaltyX {
		t.Fatal("prep missed the table's announced-X counter cost")
	}
	for i := 0; i < lands; i++ {
		sc.perm(idOf(tbl, da, "Plains"), 0)
	}
	src = sc.perm(sentry, 0)
	sl := sc.slot(src)
	for i := 0; i < ctrs; i++ {
		sc.s.perms[sl].seeCtr(sc.s)
		sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	}
	if got := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); got != ctrs {
		t.Fatalf("source has %d +1/+1 counters, want %d", got, ctrs)
	}
	return sc, tbl, sentry, plain, walker, src
}

// TestCounterXSharedWithManaX: {X} plus SubCounter<X/KIND> share one
// announcement, and gorge's xAsk min-clamps it by the counters the source has
// (Chamber Sentry, Talon of Pain). Five Plains would fund X up to 5; two
// counters bound the ask to 0..2.
func TestCounterXSharedWithManaX(t *testing.T) {
	sc, _, _, _, _, src := counterXScen(t, 5, 2)
	sc.do(t, AkActivate, uint8(src), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if d.N != 3 {
		t.Fatalf("X ask offers %d values, want 0..2 (the counters), not the mana bound", d.N)
	}
	for x := 0; x < int(d.N); x++ {
		if d.Options[x].Mode() != uint8(x) {
			t.Fatalf("option %d is X=%d", x, d.Options[x].Mode())
		}
	}
}

// TestCounterXManaBoundsBelowCounters: with fewer lands than counters the
// mana window still bounds the shared X.
func TestCounterXManaBoundsBelowCounters(t *testing.T) {
	sc, _, _, _, _, src := counterXScen(t, 1, 4)
	sc.do(t, AkActivate, uint8(src), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if d.N != 2 {
		t.Fatalf("X ask offers %d values, want 0..1 (the one land)", d.N)
	}
}

// TestAbilityCastClassSubCounterX: an announced SubCounter<X/KIND> is not a
// loyalty ability (CR 606), so an "activate a loyalty ability" listener does
// not match it; a planeswalker's [-X] carries CostLoyalty and does.
func TestAbilityCastClassSubCounterX(t *testing.T) {
	sc, tbl, sentry, plain, walker, _ := counterXScen(t, 0, 0)
	for _, c := range []struct {
		name string
		card CardID
		want bool
	}{
		{"mana X + SubCounter<X/P1P1>", sentry, false},
		{"SubCounter<X/P1P1>", plain, false},
		{"loyalty [-X]", walker, true},
	} {
		e := stackEntry{kind: seAbility, card: c.card, ab: 0}
		if f := tbl.Cards[c.card].Abilities[0].Cost.Flags & kop.CostLoyaltyX; f == 0 {
			t.Fatalf("%s: precondition: the ability is not an announced-X counter cost", c.name)
		}
		for _, cls := range []int16{kop.TrigAbCastLoyalty, kop.TrigAbCastLoyaltyOpp} {
			if got := abilityCastClass(sc.k, &e, cls); got != c.want {
				t.Errorf("%s: abilityCastClass(class %d) = %v, want %v", c.name, cls, got, c.want)
			}
		}
	}
}
