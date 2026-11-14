package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// riderBoard builds a scenario over the manarider fixture pool with one
// permanent of each named card, untapped and not sick.
func riderBoard(t *testing.T, cfg Config, names ...string) (*scen, map[string]CardID, map[string]Obj) {
	t.Helper()
	set := cardsettest.Set(t, "manarider")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, cfg)
	objs := map[string]Obj{}
	for _, n := range names {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the manarider table", n)
		}
		objs[n] = sc.perm(c, 0)
	}
	return sc, id, objs
}

// TestManaRiderActivation: the bounded riders run with the ability's mana at
// activation (CR 605.3b): a depletion land removes one counter, adds its
// production and sacrifices itself on the last counter; Grove of the
// Burnwillows' second ability gives each opponent 1 life.
func TestManaRiderActivation(t *testing.T) {
	t.Run("depletion land", func(t *testing.T) {
		sc, _, objs := riderBoard(t, Config{}, "Peat Bog")
		o := objs["Peat Bog"]
		sl := sc.slot(o)
		a, ok := sc.s.riderAbilityOf(sc.k, sl)
		if !ok || a.ManaRider != kop.ManaRiderDeplete {
			t.Fatalf("precondition: Peat Bog is not the depletion-rider shape")
		}
		sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
		if !sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), 2) {
			t.Fatal("precondition: could not put the two depletion counters on")
		}
		sc.s.perms[sl].flags &^= pfTapped // it entered tapped; it untaps on its controller's turn
		if !sc.s.riderGap(sc.k, 0, sl, 0) {
			t.Fatal("first activation refused")
		}
		if n := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); n != 1 {
			t.Fatalf("counters %d after the first activation, want 1", n)
		}
		if got := sc.s.pl[0].pool[kop.B]; got != 2 {
			t.Fatalf("{B} in pool %d, want 2", got)
		}
		if !sc.onBF(o) {
			t.Fatal("Peat Bog left the battlefield with a counter still on it")
		}
		sc.s.perms[sl].flags &^= pfTapped
		if !sc.s.riderGap(sc.k, 0, sl, 0) {
			t.Fatal("second activation refused")
		}
		if sc.onBF(o) || sc.s.objs[o].zone != kop.ZoneGraveyard {
			t.Fatalf("Peat Bog on bf %v zone %d, want sacrificed on the last counter", sc.onBF(o), sc.s.objs[o].zone)
		}
		if got := sc.s.pl[0].pool[kop.B]; got != 4 {
			t.Fatalf("{B} in pool %d, want 4", got)
		}
	})
	t.Run("any-colour depletion", func(t *testing.T) {
		sc, _, objs := riderBoard(t, Config{}, "Gemstone Mine")
		o, sl := objs["Gemstone Mine"], sc.slot(objs["Gemstone Mine"])
		a, _ := sc.s.riderAbilityOf(sc.k, sl)
		sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
		sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), 3)
		sc.s.perms[sl].flags &^= pfTapped
		if !sc.s.riderGap(sc.k, 0, sl, 2) { // the third answer: B
			t.Fatal("activation refused")
		}
		if got := sc.s.pl[0].pool[kop.B]; got != 1 {
			t.Fatalf("{B} in pool %d, want 1", got)
		}
		if n := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); n != 2 {
			t.Fatalf("mining counters %d, want 2", n)
		}
		if !sc.onBF(o) {
			t.Fatal("Gemstone Mine left the battlefield with counters left")
		}
	})
	t.Run("grove", func(t *testing.T) {
		sc, _, objs := riderBoard(t, Config{}, "Grove of the Burnwillows")
		o, sl := objs["Grove of the Burnwillows"], sc.slot(objs["Grove of the Burnwillows"])
		if _, ok := sc.s.riderAbilityOf(sc.k, sl); !ok {
			t.Fatal("precondition: Grove has no bounded rider ability")
		}
		life := sc.s.pl[1].life
		if !sc.s.riderGap(sc.k, 0, sl, 0) { // the {R} answer
			t.Fatal("activation refused")
		}
		if got := sc.s.pl[0].pool[kop.R]; got != 1 {
			t.Fatalf("{R} in pool %d, want 1", got)
		}
		if got := sc.s.pl[1].life; got != life+1 {
			t.Fatalf("opponent life %d, want %d", got, life+1)
		}
		if !sc.onBF(o) {
			t.Fatal("Grove left the battlefield")
		}
	})
}

// TestManaRiderPrefixPaysCast: the planner census never lists a bounded rider
// source, so a cast only its mana pays is offered through the potential
// play's prefix search, which activates the rider (its cost, mana and rider)
// and pays the spell. Peat Bog alone pays Black Knight's {B}{B}.
func TestManaRiderPrefixPaysCast(t *testing.T) {
	sc, id, objs := riderBoard(t, Config{NoElide: true, GorgeLowering: true}, "Peat Bog")
	o := objs["Peat Bog"]
	sl := sc.slot(o)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Peat Bog is not the depletion-rider shape")
	}
	sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)))
	sc.s.perms[sl].addCtrN(sc.s, int(a.ManaRiderCtr), 2)
	sc.s.perms[sl].flags &^= pfTapped
	h := sc.hand(id["Black Knight"], 0)
	sc.do(t, AkCast, TCode(h), scAny)
	if !sc.tapped(o) {
		t.Fatal("Peat Bog untapped: the prefix search did not activate it")
	}
	if n := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); n != 1 {
		t.Fatalf("counters %d after the cast, want 1", n)
	}
}
