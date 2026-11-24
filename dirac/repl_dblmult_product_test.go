package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The doublers' product (replDamage's dblMult, dealAsked's dblCandMult): an
// unfiltered doubler (Twinflame Tyrant) and a damage mark multiply by 2 beside
// a table's filtered or Thrice lines, and an order ask's picked candidate
// applies its own multiplier whether or not it still matches.
func TestReplDblProductUnfilteredAndThrice(t *testing.T) {
	const them = TCode(PlayerTarget0 + 1)
	t.Run("a lone unfiltered doubler doubles", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Tyrant", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
		sc := tb.scen(t)
		sc.perm(host, 0)
		if sc.k.gen.anyFiltRepl {
			t.Fatal("precondition: a lone unfiltered doubler reads the unfiltered table")
		}
		bear := sc.perm(v.GrizzlyBears, 0)
		if got := spDeal(sc, 0, bear, them, 3); got != 6 {
			t.Fatalf("3 through a lone Twinflame = %d, want 6", got)
		}
	})
	t.Run("two unfiltered doublers double twice", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Tyrant", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
		sc := tb.scen(t)
		sc.perm(host, 0)
		sc.perm(host, 0)
		bear := sc.perm(v.GrizzlyBears, 0)
		if got := spDeal(sc, 0, bear, them, 3); got != 12 {
			t.Fatalf("3 through two Twinflames = %d, want 12", got)
		}
	})
	t.Run("Thrice beside Twice", func(t *testing.T) {
		tb := newSPTable()
		tyrant := tb.host("Test Tyrant", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
		furnace := tb.host("Test Thrice", kop.Repl{Kind: kop.ReplDoubleDamageToOpps, A: 3,
			C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{})})
		sc := tb.scen(t)
		sc.perm(tyrant, 0)
		sc.perm(furnace, 0)
		if !sc.k.gen.anyFiltRepl {
			t.Fatal("precondition: the Thrice's filter puts the table on the filtered reading")
		}
		bear := sc.perm(v.GrizzlyBears, 0)
		if got := spDeal(sc, 0, bear, them, 2); got != 12 {
			t.Fatalf("2 through Twinflame and Thrice = %d, want 12 (x2 x3)", got)
		}
	})
}

// TestReplDblCandMult: the order ask's picked candidate multiplies by its own
// line (2; Thrice 3) and does not depend on a live re-match, so a candidate
// whose host a pause removed still applies (gorge applies a picked one).
func TestReplDblCandMult(t *testing.T) {
	tb := newSPTable()
	tyrant := tb.host("Test Tyrant", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
	thrice := tb.host("Test Thrice", kop.Repl{Kind: kop.ReplDoubleDamageToOpps, A: 3,
		C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{})})
	sc := tb.scen(t)
	ty := sc.perm(tyrant, 0)
	fu := sc.perm(thrice, 0)
	ro := &sc.s.g.ro
	ro.cand[0], ro.cand[1] = ty, fu
	if got := [2]int{sc.s.dblCandMult(sc.k, 0), sc.s.dblCandMult(sc.k, 1)}; got != [2]int{2, 3} {
		t.Fatalf("candidate multipliers = %v, want [2 3]", got)
	}
	// no combat ask is set, so a live re-match reads nothing: the pick still applies
	if sc.s.dblDmgLive(sc.k, sc.slot(ty), nil) {
		t.Fatal("precondition: the candidate does not re-match the (unset) ask")
	}
	sc.s.objs[ty].zone = kop.ZoneGraveyard
	sc.s.objs[fu].zone = kop.ZoneGraveyard
	if got := [2]int{sc.s.dblCandMult(sc.k, 0), sc.s.dblCandMult(sc.k, 1)}; got != [2]int{2, 3} {
		t.Fatalf("candidate multipliers after the hosts left = %v, want [2 3]", got)
	}
}
