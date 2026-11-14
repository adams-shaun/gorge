package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// courtScen: Secluded Courtyard (its creature type chosen) the only land
// of p0's, Healer's Hawk (a Bird, {W}) in hand, the oracle's lowering
// surface (Config.GorgeLowering).
func courtScen(t *testing.T, chosen string) (sc *scen, court, hawk Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Secluded Courtyard", Count: 1}, {Name: "Healer's Hawk", Count: 1}, {Name: "Plains", Count: 10}}
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
	sub := -1
	for i, n := range tbl.Subtypes {
		if n == chosen || chosen == "" && n != "Bird" && sub < 0 {
			sub = i // ("": a type other than Bird)
		}
	}
	if sub < 0 {
		t.Skipf("no subtype %s in the table", chosen)
	}
	sc = newScenTable(t, tbl, Config{NoElide: true, GorgeLowering: true})
	court = sc.perm(id["Secluded Courtyard"], 0)
	if d := sc.s.Step(); d.Kind == DkChoose { // its as-enters creature type: answered, then set
		sc.s.Apply(d.Options[0])
	}
	sc.s.perms[sc.slot(court)].chosen = objWord(uint8(sub))
	hawk = sc.hand(id["Healer's Hawk"], 0)
	if sc.s.scriptAnyR(sc.k, 0).any() || !sc.k.restrSrc {
		t.Fatal("the Courtyard is an any-colour gap, not a restricted planner unit")
	}
	return sc, court, hawk
}

// TestCourtyardGap: the planner counts Secluded
// Courtyard's restricted any-colour ability as an alternative of its census
// unit, admitted per payment (RestrictValidMatches), so the Hawk, which the
// Courtyard's {C} cannot pay, is a planned cast when the Courtyard's chosen
// type (Bird) admits it: the plan taps the Courtyard for {W} at once, no
// script and no floating restricted mana. With another type chosen the
// Courtyard makes {C} for the Hawk and it is not offered.
func TestCourtyardGap(t *testing.T) {
	sc, court, hawk := courtScen(t, "Bird")
	s := sc.s
	d := s.Step()
	if !castOffered(&d, hawk) {
		t.Fatalf("Hawk not offered (Bird chosen): decision kind %d, %d options", d.Kind, d.N)
	}
	s.Apply(MakeAction(AkCast, TCode(hawk), 0, 0))
	s.Step()
	if !sc.tapped(court) {
		t.Fatal("the Courtyard is not tapped by the plan")
	}
	if s.rbAny(0) || s.pl[0].pool != ([kop.NumManaKinds]uint8{}) || s.sp != 1 {
		t.Fatalf("after the Hawk: restricted %v pool %v stack %d, want nothing floating and the Hawk on the stack", s.rb[0], s.pl[0].pool, s.sp)
	}

	sc, _, hawk = courtScen(t, "")
	d = sc.s.Step()
	if castOffered(&d, hawk) {
		t.Fatal("Hawk offered with another type chosen: the Courtyard's restricted {W} is withheld from it")
	}
}

// TestCourtyardGapAllocs: the priority step that prices the Courtyard's
// restricted alternative (restrSrcAffords) allocates nothing, whether the
// Hawk's payment admits it (Bird chosen) or withholds it (another type).
func TestCourtyardGapAllocs(t *testing.T) {
	for _, chosen := range []string{"Bird", ""} {
		sc, _, _ := courtScen(t, chosen)
		var base, s State
		base.CopyFrom(sc.s)
		if a := testing.AllocsPerRun(20, func() { s.CopyFrom(&base); s.Step() }); a != 0 {
			t.Errorf("chosen %q: %.0f allocations a priority step, want 0", chosen, a)
		}
	}
}

// TestVillageFundsCreature: Mudflat Village's "{T}: Add {B}. Spend this mana
// only to cast a creature spell" (a second mana ability beside its {C}) is a
// planner source for a creature cast (gorge's census lists it and
// the descriptor admits the spell), never for a noncreature one. Two Villages
// are the only lands: Agate-Blade Assassin ({1}{B}) is a cast they pay ({C}
// and the restricted {B}), Savor (a {1}{B} instant) is no cast at all.
func TestVillageFundsCreature(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Mudflat Village", Count: 2}, {Name: "Agate-Blade Assassin", Count: 1}, {Name: "Savor", Count: 1}, {Name: "Plains", Count: 10}})
	s := sc.s
	v1 := sc.perm(id["Mudflat Village"], 0)
	v2 := sc.perm(id["Mudflat Village"], 0)
	assassin := sc.hand(id["Agate-Blade Assassin"], 0)
	savor := sc.hand(id["Savor"], 0)
	if rec := sc.k.gen.restrRecOf(id["Mudflat Village"]); rec == nil || rec.kind != rkWiden {
		t.Fatal("the Village is not a widening restricted source")
	}
	d := s.Step()
	if !castOffered(&d, assassin) {
		t.Fatalf("Agate-Blade Assassin not offered on the Villages' {C} and restricted {B}: %d options", d.N)
	}
	if castOffered(&d, savor) {
		t.Fatal("Savor offered on a creature-only {B}")
	}
	s.Apply(MakeAction(AkCast, TCode(assassin), 0, 0))
	s.Step()
	if !sc.tapped(v1) || !sc.tapped(v2) || s.sp != 1 || s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("after the Assassin: Villages tapped %v %v, stack %d, pool %v; want both tapped and the Assassin on the stack", sc.tapped(v1), sc.tapped(v2), s.sp, s.pl[0].pool)
	}
}

// TestCourtyardsKeepTheirOwnChoice: mana floated from a Courtyard that chose Goblin
// stays spendable only on Goblins while another Courtyard that chose Bird sits tapped
// beside it (CR 106.6: the restriction is the source's, as it was made).
func TestCourtyardsKeepTheirOwnChoice(t *testing.T) {
	sc, court, hawk := courtScen(t, "")
	s := sc.s
	other := sc.perm(s.objs[court].card, 0)
	if d := s.Step(); d.Kind == DkChoose {
		s.Apply(d.Options[0])
	}
	bird := -1
	for i, n := range sc.k.t.Subtypes {
		if n == "Bird" {
			bird = i
		}
	}
	s.perms[sc.slot(other)].chosen = objWord(uint8(bird))
	var f kop.FilterID
	for _, a := range sc.k.t.Cards[s.objs[court].card].Abilities {
		if a.Kind == kop.AbMana && a.ManaRestrict != 0 {
			f = a.ManaRestrict
		}
	}
	if !s.floatRestrFrom(sc.k, 0, sc.slot(court), f, int(kop.W), 1) {
		t.Fatal("the Courtyard's mana did not float restricted")
	}
	s.perms[sc.slot(court)].flags |= pfTapped
	s.perms[sc.slot(other)].flags |= pfTapped
	if d := s.Step(); castOffered(&d, hawk) {
		t.Fatalf("the Hawk is offered from mana of a Courtyard that did not choose Bird: %v", d.Options[:d.N])
	}
}
