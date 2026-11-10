package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ptEqGtScen is a scenario over the ptcompare fixture pool holding the named cards
// (one Forest to fill the libraries): the ids by name.
func ptEqGtScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Forest", Count: 1}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := cardsettest.Set(t, "ptcompare").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, n := range names {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s is not in the matchup's table", n)
		}
	}
	return newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true}), id
}

// ptEqGtDrain passes priority until the triggers queued so far have resolved.
func ptEqGtDrain(t *testing.T, sc *scen) {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d while draining the stack", d.Kind)
		}
		if sc.s.sp == 0 && sc.s.npend == 0 {
			return
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("the stack never emptied")
}

// TestPTCompareEqGtKernel: Amalia Benavides Aguirre destroys the other
// creatures after she explores only if her power is exactly 20 (PredPowerGEV
// PredN 3); Hulkling, Burgeoning Bruiser (a 2/3) takes a +1/+1 counter when
// another creature you control enters with greater power (3/1) or greater
// toughness (1/4, PredToughnessLEV PredN 3), and not with an equal or smaller
// one (2/2).
func TestPTCompareEqGtKernel(t *testing.T) {
	const amalia, bystander, atlas = "Amalia Benavides Aguirre", "Colossadactyl", "Agent of Atlas"
	for _, c := range []struct {
		name  string
		power int
		dies  bool
	}{{"nineteen", 19, false}, {"twenty", 20, true}, {"twenty-one", 21, false}} {
		t.Run(amalia+" "+c.name, func(t *testing.T) {
			sc, id := ptEqGtScen(t, amalia, bystander)
			am := sc.perm(id[amalia], 0)
			mine, theirs := sc.perm(id[bystander], 0), sc.perm(id[bystander], 1)
			sl := sc.slot(am)
			sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrP1P1), c.power-2)
			if pw, _ := sc.s.ptGen(sc.k, sl); pw != c.power {
				t.Fatalf("Amalia's power is %d, want %d", pw, c.power)
			}
			sc.s.gainLife(sc.k, 0, 1)
			ptEqGtDrain(t, sc)
			if !sc.onBF(am) {
				t.Fatal("Amalia left the battlefield: she destroys the OTHER creatures")
			}
			if got := !sc.onBF(mine) && !sc.onBF(theirs); got != c.dies {
				t.Errorf("power %d: both other creatures destroyed = %v (mine on bf %v, theirs on bf %v), want %v",
					c.power, got, sc.onBF(mine), sc.onBF(theirs), c.dies)
			}
			if !c.dies && (!sc.onBF(mine) || !sc.onBF(theirs)) {
				t.Errorf("power %d: a creature died", c.power)
			}
		})
	}
	const hulk = "Hulkling, Burgeoning Bruiser"
	for _, c := range []struct {
		name string
		want int
	}{{"Misty Knight, Hero for Hire", 1}, {"Echo, Perceptive Prodigy", 1}, {atlas, 0}, {"Kree Sentinel", 1}} {
		t.Run(hulk+" "+c.name, func(t *testing.T) {
			sc, id := ptEqGtScen(t, hulk, c.name)
			hk := sc.perm(id[hulk], 0)
			if pw, tg := sc.s.ptGen(sc.k, sc.slot(hk)); pw != 2 || tg != 3 {
				t.Fatalf("Hulkling is %d/%d, want 2/3", pw, tg)
			}
			o := sc.obj(id[c.name], 0)
			sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
			ptEqGtDrain(t, sc)
			if got := sc.pceCount(hk, kop.CtrP1P1); got != c.want {
				t.Errorf("%s entered: Hulkling has %d +1/+1 counters, want %d", c.name, got, c.want)
			}
		})
	}
}
