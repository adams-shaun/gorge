package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// obsWinScen: a live spell build, one Forest marked as an observed mana
// source (Barracks of the Thousand's "mana from this land" reader) and a
// Plains, with cast-source taps recorded.
func obsWinScen(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Cackling Prowler", Count: 1}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4}})
	forest := sc.perm(id["Forest"], 0)
	plains := sc.perm(id["Plains"], 0)
	sc.k.cards[id["Forest"]].castSrc |= csObserved
	sc.k.gen.castTapOn = true
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, card: id["Cackling Prowler"], ctrl: 0}}
	sc.s.ctl = ctlBuild
	return sc, forest, plains
}

// TestObsWinPaymentTapCounts: an observed source first tapped by the
// payment's own window (a free or lowered cast's CR 601.2g window) spends its
// mana on the cast: obsSl stays and the trigger fires. Before, the baseline
// was the zero array and obsWinSettle cleared obsSl.
func TestObsWinPaymentTapCounts(t *testing.T) {
	sc, forest, _ := obsWinScen(t)
	s := sc.s
	pool, ok := s.obsWinPool(0)
	if ok {
		t.Fatal("a baseline taken with no pre-payment observed tap is valid")
	}
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}}
	s.payCastWindow(sc.k, 0, &cost, false, false)
	if !sc.tapped(forest) || s.bld().obsSl != s.objs[forest].slot+1 {
		t.Fatalf("payment window: Forest tapped %v, obsSl %d: want the Forest tapped and recorded", sc.tapped(forest), s.bld().obsSl)
	}
	if s.bld().obsWin&obsWinTapped != 0 {
		t.Fatal("a payment-window tap marked the observed source as floated before the payment")
	}
	s.obsWinSettle(sc.k, 0, pool, ok)
	if s.bld().obsSl == 0 {
		t.Fatal("obsWinSettle cleared the observed source the payment's window tapped and spent")
	}
}

// TestObsWinFloatedUnspent: an observed source tapped before the payment
// whose mana the payment did not spend (pair 368: Barracks' {W} floating
// beside an X=0 {G} cost) is not recorded; one whose colour the payment
// spends is.
func TestObsWinFloatedUnspent(t *testing.T) {
	for _, spend := range []bool{false, true} {
		sc, forest, _ := obsWinScen(t)
		s := sc.s
		s.noteTapSrc(sc.k, &sc.k.cards[s.cardAt(s.objs[forest].slot)], s.objs[forest].slot)
		s.bld().obsWin |= obsWinTapped
		s.pl[0].pool[kop.G] = 1 // the Forest's floated {G}
		pool, ok := s.obsWinPool(0)
		if !ok || s.bld().obsSl == 0 {
			t.Fatalf("spend %v: baseline ok %v obsSl %d: want a valid baseline and the source recorded", spend, ok, s.bld().obsSl)
		}
		if spend {
			s.pl[0].pool[kop.G] = 0 // the payment spent the floated {G}
		} // else the cast paid with other mana (a land tapped in the payment) and the {G} floats on
		s.obsWinSettle(sc.k, 0, pool, ok)
		if got := s.bld().obsSl != 0; got != spend {
			t.Fatalf("spend %v: obsSl kept %v", spend, got)
		}
	}
}
