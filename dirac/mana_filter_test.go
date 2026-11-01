package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grottoScen: Ravine Raider ({1}{B}: +1/+1) and Hidden Grotto ("{T}: Add
// {C}"; "{1}, {T}: Add one mana of any color") beside the given lands.
func grottoScen(t *testing.T, lands ...string) (*scen, Obj, Obj, []Obj) {
	t.Helper()
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Ravine Raider", Count: 1}, {Name: "Hidden Grotto", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8}})
	var ls []Obj
	for _, n := range lands {
		ls = append(ls, sc.perm(id[n], 0))
	}
	grotto := sc.perm(id["Hidden Grotto"], 0)
	raider := sc.perm(id["Ravine Raider"], 0)
	return sc, raider, grotto, ls
}

func raiderPump(sc *scen, raider Obj) (Action, bool) {
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && Obj(a.Source()) == raider {
			return a, true
		}
	}
	return 0, false
}

// TestGrottoFilterPaysRaider: with two Plains and Hidden Grotto, Ravine
// Raider's {1}{B} is payable only through the Grotto's filter (CR 605.3a,
// 601.2g). gorge's planner census misses the filter, so the play is a
// scripted potential play (rules potentialPrefixScript): the first Plains
// funds the filter's {1}, the filter makes {B} (W and U are tried first
// and price nothing), and the planner pays the rest from the pool and the
// other Plains (BLB pairs 237, 436, 520, 1261, 1288).
func TestGrottoFilterPaysRaider(t *testing.T) {
	sc, raider, grotto, ls := grottoScen(t, "Plains", "Plains")
	a, ok := raiderPump(sc, raider)
	if !ok {
		t.Fatal("Ravine Raider's {1}{B} not offered over Hidden Grotto's filter")
	}
	sc.s.Apply(a)
	sc.s.Step()
	for _, o := range append(ls, grotto) {
		if !sc.tapped(o) {
			t.Errorf("%s untapped after the activation", sc.k.t.Cards[sc.s.objs[o].card].Name)
		}
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the activation, want empty", pool)
	}
}

// TestGrottoFilterNeedsFunding: the relaxed census (the filter as a free
// any-colour unit, its fee charged) pays {1}{B} from one Plains and the
// Grotto, but no real sequence does: the Plains funds the filter and
// nothing is left for the {1}. Not offered.
func TestGrottoFilterNeedsFunding(t *testing.T) {
	sc, raider, _, _ := grottoScen(t, "Plains")
	if _, ok := raiderPump(sc, raider); ok {
		t.Fatal("Ravine Raider's {1}{B} offered with one Plains and Hidden Grotto")
	}
}
