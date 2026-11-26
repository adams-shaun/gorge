package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAltGrantCountBound: seven free alternative-cost grants on the
// battlefield list seven granted casts of a hand creature, at AltCostIndex 1
// through 7; an eighth cannot be indexed (maxAltGrants), so the game stalls
// loud (StallZeroAltShared) and lists no granted cast, never a truncated set
// (cast_altgrant.go offerAltGrants).
func TestAltGrantCountBound(t *testing.T) {
	run := func(t *testing.T, n int) (granted int, stalled bool) {
		t.Helper()
		tbl, ids := kcapsTable(t, kop.CardDef{Name: "Test Grant", Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatAlternativeCost, StFlags: kop.StAltGrant}}}})
		tbl.Exprs = []kop.Expr{} // non-nil: the generated interpreter
		sc := newScenTable(t, tbl, Config{NoElide: true})
		for i := 0; i < n; i++ {
			sc.perm(ids[0], 0)
		}
		sc.hand(v.GrizzlyBears, 0)
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Mode() == 0 && a.Target() > castAltIdxTgt {
				granted++
			}
		}
		return granted, sc.s.Stalled() && sc.s.StallReason() == StallZeroAltShared
	}
	if g, st := run(t, 7); g != 7 || st {
		t.Fatalf("seven grants: %d granted casts listed (want 7), stalled %v", g, st)
	}
	if g, st := run(t, 8); g != 0 || !st {
		t.Fatalf("eight grants: %d granted casts listed (want 0), stalled %v (want true)", g, st)
	}
}
