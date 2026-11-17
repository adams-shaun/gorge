package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// rainFilthScen builds the dr-mbc-bbba shape: p0 holds Saprazzan Skerry (a
// bounded deplete rider), a tapped Island and a tapped, summoning-sick Swamp
// under Rain of Filth's granted "Sacrifice this land: Add {B}", with spell in
// hand. nCtr is the Skerry's depletion counters (1 after its one use in the
// recorded game), tapped whether the Island and the Swamp read.
func rainFilthScen(t *testing.T, spell string, nCtr int, tapped bool) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Rain of Filth", Count: 1},
		{Name: "Saprazzan Skerry", Count: 1},
		{Name: "Souldrinker", Count: 1},
		{Name: "Wight of Precinct Six", Count: 1},
		{Name: "Archfiend of Sorrows", Count: 1},
		{Name: "Island", Count: 2},
		{Name: "Swamp", Count: 2},
		{Name: "Plains", Count: 10},
	}
	sc, id := newSetScenTweak(t, "mbc", deck, nil)
	skerry := sc.perm(id["Saprazzan Skerry"], 0)
	sl := sc.slot(skerry)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Saprazzan Skerry is not the bounded-rider shape")
	}
	pm := &sc.s.perms[sl]
	pm.subCtrN(sc.s, int(a.ManaRiderCtr), pm.ctrN(sc.s, int(a.ManaRiderCtr)))
	if nCtr > 0 && !pm.addCtrN(sc.s, int(a.ManaRiderCtr), nCtr) {
		t.Fatal("precondition: could not put the depletion counters on")
	}
	pm.flags &^= pfTapped
	isl := sc.perm(id["Island"], 0)
	swp := sc.perm(id["Swamp"], 0)
	rof := sc.hand(id["Rain of Filth"], 0)
	sc.hand(id[spell], 0)
	sc.hand(id["Wight of Precinct Six"], 0)
	sc.do(t, AkCast, TCode(rof), scAny) // the Swamp pays its {B}
	sc.resolveAll(t)
	if gi := sc.s.grantIdx(sc.k, sc.slot(skerry)); gi < 0 {
		t.Fatal("precondition: Rain of Filth's grant is not live on the Skerry")
	}
	if tapped {
		sc.s.perms[sc.slot(isl)].flags |= pfTapped
		sc.s.perms[sc.slot(swp)].flags |= pfTapped | pfSick
	}
	return sc, skerry
}

// offered reports whether the priority decision offers the named card's cast.
func riderGrantCastOffered(sc *scen, name string) bool {
	d := sc.s.Step()
	for _, x := range d.Options[:d.N] {
		if x.Kind() == AkCast && sc.k.t.Cards[sc.s.objs[x.Source()].card].Name == name {
			return true
		}
	}
	return false
}

// TestRiderGrantRiderGapPaysCast: under Rain of Filth, every land p0 controls
// also has "Sacrifice this land: Add {B}", and Saprazzan Skerry's own bounded
// rider ({T}, remove a depletion counter: Add {U}{U}) is a census gap no walk
// lists. The rider's mana is still the script route's (scriptTapsDefer): the
// one-counter Skerry, the Island and the Swamp pay Souldrinker's {3}{B} (CR
// 605.1a, 605.3b; dr-mbc-bbba pair 47, decision 189). Wight of Precinct Six's
// {1}{B} keeps its plain offer (the grants' census units survive the gap).
func TestRiderGrantRiderGapPaysCast(t *testing.T) {
	sc, _ := rainFilthScen(t, "Souldrinker", 1, true)
	if !riderGrantCastOffered(sc, "Souldrinker") {
		t.Fatal("Souldrinker's {3}{B} not offered over the Skerry's rider and the grants")
	}
	if !riderGrantCastOffered(sc, "Wight of Precinct Six") {
		t.Fatal("Wight of Precinct Six's {1}{B} no longer offered from the grants' census units")
	}
}

// TestRiderGrantRiderGapNeedsTheCounter: with no depletion counter left the
// rider's {T}, remove a depletion counter cost is unpayable, and the grants
// alone make three {B} -- short of Souldrinker's four.
func TestRiderGrantRiderGapNeedsTheCounter(t *testing.T) {
	sc, _ := rainFilthScen(t, "Souldrinker", 0, true)
	if riderGrantCastOffered(sc, "Souldrinker") {
		t.Fatal("Souldrinker's {3}{B} offered with the rider's counter spent")
	}
}

// TestRiderGrantRiderGapPaysThePool: applying the offered cast pays it --
// the rider's {U}{U} floats over the script route's prefix, and the payment
// plan's phase 2 activates the granted sacrifices of the printed tap sources
// it cannot tap (planGrantSacUnit), draining the pool (CR 605.1a, 605.3b).
// Without the plan units the kernel panics "autopay found no plan"
// (dr-mbc-bbba pair 47, decision 190).
func TestRiderGrantRiderGapPaysThePool(t *testing.T) {
	sc, _ := rainFilthScen(t, "Souldrinker", 1, true)
	d := sc.s.Step()
	var cast Action
	for _, x := range d.Options[:d.N] {
		if x.Kind() == AkCast && sc.k.t.Cards[sc.s.objs[x.Source()].card].Name == "Souldrinker" {
			cast = x
		}
	}
	if cast == 0 {
		t.Fatal("precondition: cast(Souldrinker) not offered")
	}
	var lands []Obj
	for o, ob := range sc.s.objs {
		if ob.zone == kop.ZoneBattlefield && (sc.k.t.Cards[ob.card].Name == "Island" || sc.k.t.Cards[ob.card].Name == "Swamp") {
			lands = append(lands, Obj(o))
		}
	}
	sc.s.Apply(cast)
	for guard := 0; guard < 16 && !sc.s.over; guard++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.over {
		t.Fatal("the game ended during the cast")
	}
	if pool := sc.s.pl[0].pool; pool != ([6]uint8{}) {
		t.Fatalf("pool %v left after the cast, want empty", pool)
	}
	sac := 0
	for _, o := range lands {
		if sc.s.objs[o].zone == kop.ZoneGraveyard {
			sac++
		}
	}
	if sac == 0 {
		t.Error("no grant land sacrificed for the cast")
	}
}

// TestRiderGrantRiderGapGrantDiesWithTheSource: the rider's last activation
// sacrifices the Skerry, its grant with it, so the real maximum is {U}{U} and
// two {B}: Archfiend of Sorrows's {2}{B}{B}{B} (five mana, three black pips)
// is beyond it, however the relaxed census counts (its gaps walk reads the
// rider's {U}{U} beside all three grants). gorge's search refuses it the same
// way, so the kernel must too.
func TestRiderGrantRiderGapGrantDiesWithTheSource(t *testing.T) {
	sc, _ := rainFilthScen(t, "Archfiend of Sorrows", 1, true)
	if riderGrantCastOffered(sc, "Archfiend of Sorrows") {
		t.Fatal("Archfiend of Sorrows's {2}{B}{B}{B} offered: the Skerry's grant dies with the rider's activation")
	}
}
