package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWindowColOwedCountsTheRestrictedManaOnce: a window source's colour ask measures
// the cost less the floating restricted mana the payment may spend (windowOwedCost),
// and the helpers that read the demand (windowDemand, windowUseful) price that mana
// from the pool themselves. Handed the already shortened cost they count it twice, so
// {U}{U} beside one admitted restricted {U} read as owing nothing and an any-colour
// ability (Starting Town's) answered {C} instead of the {U} still owed.
func TestWindowColOwedCountsTheRestrictedManaOnce(t *testing.T) {
	sc, _, _ := restrDemandScen(t)
	s, k := sc.s, sc.k
	s.pl[0].pool = [kop.NumManaKinds]uint8{}
	// a source whose own colour is not the one owed, so only the any-colour ability can answer it
	var plains CardID
	for i := range k.t.Cards {
		if k.t.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	sl := sc.slot(sc.perm(plains, 0))
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 2}}
	if w := s.windowDemand(k, 0, &cost); w != 1<<kop.U {
		t.Fatalf("precondition: demand %#x, want the second {U} owed beside the restricted one", w)
	}
	var owed kop.ManaCost
	short := *s.windowOwedCost(k, 0, &cost, &owed)
	if short.Pips[kop.U] != 1 {
		t.Fatalf("precondition: owed {U} pips %d, want 1", short.Pips[kop.U])
	}
	if w := s.windowDemand(k, 0, &short); w != 0 {
		t.Fatalf("precondition: demand %#x on the shortened cost, want 0 (the double count)", w)
	}
	pc := k.cards[s.cardAt(sl)]
	pc.painCol, pc.manaCol, pc.painOnly = kop.ColAny, 0, false
	k.painMana = true
	if col := s.windowColOwed(k, 0, &cost, &short, sl, &pc, false, false); col != kop.U {
		t.Fatalf("an any-colour ability answers colour %d, want {U} (%d), the pip still owed", col, kop.U)
	}
}
