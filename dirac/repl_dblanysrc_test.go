package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The widened DamageDone doubler (dirac/repl_multipliers.go): a filter that
// names no controller (ValidSource$ Card,Emblem — any card or emblem, any
// controller's) doubles a foreign controller's source too; the doubler's own
// Thrice (Repl.A 3) multiplies by 3; and the ValidTarget$ You / Card.Self
// words (kop.ReplDmgYou / ReplDmgSelf, Goldnight Castigator) narrow the
// recipients to the controller and the host itself. The any-ctrl filter is
// gorge's bare Card/Emblem words (damageReplacementMatches's matchesSpec), so
// the scenarios read the source by what it is, never by its controller.
func TestReplDblAnySrcScansBothSeats(t *testing.T) {
	const you, them = TCode(PlayerTarget0), TCode(PlayerTarget0 + 1)
	t.Run("any-ctrl doubler on the other seat doubles a foreign source", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Doubler", kop.Repl{Kind: kop.ReplDoubleDamageToOpps,
			C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{})})
		sc := tb.scen(t)
		if sc.perm(host, 1) == 0 {
			t.Fatal("precondition: the doubler host is not on seat 1")
		}
		if sc.k.gen == nil || !sc.k.gen.anyDblAnySrc {
			t.Fatal("precondition: the gen flag anyDblAnySrc is not set")
		}
		bear := sc.perm(v.GrizzlyBears, 0)
		if got := spDeal(sc, 0, bear, you, 3); got != 6 {
			t.Fatalf("seat 0's source dealt to seat 0 = %d, want 6 (the seat 1 doubler matches any source)", got)
		}
	})
	t.Run("the unfiltered reading stays the source controller's", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Tyrant", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
		sc := tb.scen(t)
		sc.perm(host, 1)
		bear := sc.perm(v.GrizzlyBears, 0)
		if got := spDeal(sc, 0, bear, them, 3); got != 3 {
			t.Fatalf("a foreign source by the seat 1 unfiltered doubler = %d, want 3", got)
		}
	})
	t.Run("a YouCtrl-filtered doubler never matches a foreign source", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test City", kop.Repl{Kind: kop.ReplDoubleDamageToOpps,
			C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{Ctrl: kop.CtrlYou})})
		sc := tb.scen(t)
		sc.perm(host, 1)
		if sc.k.gen.anyDblAnySrc {
			t.Fatal("precondition: a YouCtrl-only doubler widens no scan")
		}
		bear := sc.perm(v.GrizzlyBears, 0)
		if got := spDeal(sc, 0, bear, them, 3); got != 3 {
			t.Fatalf("a foreign source by a YouCtrl doubler on seat 1 = %d, want 3", got)
		}
		giant := sc.perm(v.HillGiant, 1)
		if got := spDeal(sc, 1, giant, them, 3); got != 6 {
			t.Fatalf("seat 1's own source = %d, want 6", got)
		}
	})
}

func TestReplDblThrice(t *testing.T) {
	const you = TCode(PlayerTarget0)
	tb := newSPTable()
	host := tb.host("Test Doubler", kop.Repl{Kind: kop.ReplDoubleDamageToOpps, A: 3,
		C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{})})
	sc := tb.scen(t)
	sc.perm(host, 0)
	bear := sc.perm(v.GrizzlyBears, 0)
	if got := spDeal(sc, 1, bear, you, 5); got != 15 {
		t.Fatalf("5 damage through the Thrice doubler = %d, want 15", got)
	}
}

func TestReplDblYouSelf(t *testing.T) {
	const you, them = TCode(PlayerTarget0), TCode(PlayerTarget0 + 1)
	t.Run("ValidTarget You", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Goldnight", kop.Repl{Kind: kop.ReplDoubleDamageToOpps,
			C: kop.ReplDmgYou, Filter: tb.filter(kop.Filter{})})
		sc := tb.scen(t)
		sc.perm(host, 0)
		bear := sc.perm(v.GrizzlyBears, 1)
		if got := spDeal(sc, 1, bear, you, 4); got != 8 {
			t.Fatalf("damage to the doubler's controller = %d, want 8", got)
		}
		if got := spDeal(sc, 1, bear, them, 4); got != 4 {
			t.Fatalf("damage to the other player = %d, want 4", got)
		}
		giant := sc.perm(v.HillGiant, 0)
		if got := spDeal(sc, 1, bear, TCode(giant), 4); got != 4 {
			t.Fatalf("damage to the controller's own creature = %d, want 4 (You names players only)", got)
		}
	})
	t.Run("ValidTarget Card.Self", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Goldnight", kop.Repl{Kind: kop.ReplDoubleDamageToOpps,
			C: kop.ReplDmgSelf, Filter: tb.filter(kop.Filter{})})
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		bear := sc.perm(v.GrizzlyBears, 1)
		if got := spDeal(sc, 1, bear, TCode(h), 4); got != 8 {
			t.Fatalf("damage to the host itself = %d, want 8", got)
		}
		giant := sc.perm(v.HillGiant, 0)
		if got := spDeal(sc, 1, bear, TCode(giant), 4); got != 4 {
			t.Fatalf("damage to another own creature = %d, want 4", got)
		}
		if got := spDeal(sc, 1, bear, you, 4); got != 4 {
			t.Fatalf("damage to the controller = %d, want 4 (Card.Self names the host only)", got)
		}
	})
}
