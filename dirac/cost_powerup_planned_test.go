package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPowerUpPlannedCostComposesStatics: gorge's planner prices a Power-up
// ability's printed cost without the entry-turn reduction (CR 702.193b) but
// with the cost statics (offerCostFor, CR 601.2f, 602.2b). Under the
// Hulk-shaped reducer ({3} less), Brave Brawler's {4}{W} is planned as {1}{W},
// which two untapped Plains pay with no floating mana: the ability is offered
// (She-Hulk, Jade Defender under Hulk, Gamma Goliath, std-msh pair 276).
func TestPowerUpPlannedCostComposesStatics(t *testing.T) {
	sc, reducer, _, brawler, _ := powerUpReducerScen(t)
	if !sc.onBF(reducer) || !sc.onBF(brawler) {
		t.Fatal("the reducer and Brave Brawler are not both on the battlefield")
	}
	pl := sc.s.objs[sc.s.pl[0].lib[0]].card // the library is all Plains
	sc.perm(pl, 0)
	sc.perm(pl, 0)
	sc.nextTurn() // nothing entered this turn: no CR 702.193b reduction
	a := powerupAbility(t, sc, sc.s.objs[brawler].card)
	if a.Cost.Flags&kop.CostPowerUp == 0 || a.Cost.Mana.MV() != 5 {
		t.Fatalf("Brave Brawler's ability: flags %#x, mana value %d; want a Power-up costing 5", a.Cost.Flags, a.Cost.Mana.MV())
	}
	if got := sc.s.powerUpManaObj(sc.k, brawler, a); got != a.Cost.Mana {
		t.Fatalf("Brave Brawler did not enter this turn, yet its Power-up is reduced to %v", got)
	}
	if got := sc.s.abReduced(sc.k, a, sc.slot(brawler), 0, a.Cost.Mana); got.MV() != 2 {
		t.Fatalf("the reducer prices Brave Brawler's Power-up at mana value %d, want 2", got.MV())
	}
	if sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("floating mana %v: the lands alone must pay", sc.s.pl[0].pool)
	}
	d := sc.s.Step()
	if d.Kind != DkPriority || !powerupActivation(&d, brawler) {
		t.Fatalf("two Plains under the reducer: Brave Brawler's {1}{W} Power-up not offered (kind %d): %v", d.Kind, d.Options[:d.N])
	}
}
