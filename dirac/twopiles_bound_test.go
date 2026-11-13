package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoPilesRevealBound: OpTwoPiles' split rides armRes.pileOrd, twelve
// slots, so a reveal of twelve is split as usual (by the opponent under
// kop.TpOppSeparates), and a reveal of thirteen (Epiphany at the Drownyard's
// X plus one) stalls StallLook, a declared capacity, before any ask
// (act_twopiles.go pilesTooWide).
func TestTwoPilesRevealBound(t *testing.T) {
	run := func(t *testing.T, n int) (sc *scen, split bool, by uint8) {
		t.Helper()
		tbl, ids := kcapsTable(t, kop.CardDef{Name: "Test Piles", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
				{Op: kop.OpPeek, Obj: kop.RefController, A: kop.Const(n), C: int16(kop.PeekPublic | kop.PeekRemember)},
				{Op: kop.OpTwoPiles, Obj: kop.RefController, B: kop.Const(kop.TpOppSeparates), C: int16(kop.ZoneHand)},
			}}}})
		tbl.Exprs = []kop.Expr{} // non-nil: the generated interpreter
		sc = newScenTable(t, tbl, Config{NoElide: true})
		if lib := sc.s.pl[0].libLen(); lib < n {
			t.Fatalf("library %d, want at least %d", lib, n)
		}
		c := sc.hand(ids[0], 0)
		sc.stepTo(t, 40, func() bool { return split || sc.s.Stalled() }, func(d *Decision) (Action, bool) {
			if d.Kind == DkChoose && d.Sub == ChGenPileSplit {
				split, by = true, d.Player
				if got := int(sc.s.g.nrem); got != n {
					t.Fatalf("%d remembered at the split, want %d", got, n)
				}
				return 0, false
			}
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(c)) {
					return a, true
				}
			}
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Kind() == AkPass {
					return d.Options[i], true
				}
			}
			return 0, false
		})
		return sc, split, by
	}
	sc, split, by := run(t, 12)
	if !split || sc.s.Stalled() {
		t.Fatalf("12 revealed: split asked %v, stalled %v (%s)", split, sc.s.Stalled(), StallName(sc.s.StallReason()))
	}
	if by != 1 {
		t.Fatalf("12 revealed: the split asked p%d, want the opponent p1", by)
	}
	sc, split, _ = run(t, 13)
	if split {
		t.Fatal("13 revealed: a split was asked past pileOrd")
	}
	if got := sc.s.StallReason(); got != StallLook {
		t.Fatalf("13 revealed: stall %q, want look", StallName(got))
	}
}
