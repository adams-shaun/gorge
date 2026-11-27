package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The TargetedByTarget$Valid count head (gorge engine/effects/count_body.go):
// the object targets, matching the spec, of the spell or ability the resolving
// source targets -- one level past the kernel's own entryTgtClause walk. Not of
// This World gates its ReduceCost amount on Count$Compare CheckTgt GE1.7.0 over
// TargetedByTarget$Valid Card.powerGE7+YouCtrl, so the read must be live at the
// offer gate and at the CR 601.2f reprice, not just at the resolution.

// TestTargetedByTargetNestedCountLowers: the carrier lowers, and its ReduceCost
// amount is the dynamic compare over the nested-target count: a threshold of 1
// (GE1), the arms 7 and 0, and an ExCountTgtEntry node carrying the spec's
// filter. The assertion fails if the head lowers to any other shape (an
// unmodelled zero would drop the reduction silently).
func TestTargetedByTargetNestedCountLowers(t *testing.T) {
	kp := kfhBuild(t, []string{"Not of This World"}, nil)
	kp.accepted(t, "Not of This World")
	def := kp.built["Not of This World"].def
	var st *kop.Static
	for i := range def.Abilities {
		a := &def.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost {
			st = &a.Static
		}
	}
	if st == nil {
		t.Fatal("the card carries no ReduceCost static")
	}
	if st.StFlags&kop.StDynamic == 0 {
		t.Fatalf("the SVar amount is not dynamic: A=%d VA=%+v", st.A, st.VA)
	}
	top := kp.expr(t, st.VA)
	if top.Kind != kop.ExCompare {
		t.Fatalf("the amount's top node is %v, want Compare", top.Kind)
	}
	if top.N != 1 {
		t.Errorf("the threshold is %d, want 1 (GE1)", top.N)
	}
	nested := kp.nestedTgtCount(t, st.VA)
	if nested == nil {
		t.Fatalf("the amount's expr tree carries no nested-target count: top=%+v", top)
	}
	if nested.F == 0 {
		t.Errorf("the nested count has no filter, so it would count every inner target")
	}
	if arms := kp.expr(t, top.Sub[1]); arms.Kind != kop.ExBranches || len(kp.arms(t, arms)) != 2 {
		t.Fatalf("the compare's arms are %+v, want two branches", arms)
	} else if got := kp.arms(t, arms); got[0] != 7 || got[1] != 0 {
		t.Errorf("the arms are %v, want {7} then 0", got)
	}
}

// TestTargetedByTargetValueArms: the modelled read is Valid with a non-empty
// object spec. A player-only spec is refused rather than lowered to a filter
// that silently matches nothing (an unmodelled zero would drop the reduction),
// and so is any other property or an empty spec, per the unmodelled-property
// convention gorge's own arm follows.
func TestTargetedByTargetValueArms(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}}
	v, err := lc.svarValue("TargetedByTarget$Valid Card.powerGE7+YouCtrl", 0)
	if err != nil || v.Kind != vCountTgtEntry || v.F == nil {
		t.Fatalf("Valid Card.powerGE7+YouCtrl: got %s (%v), want CountTgtEntry with a filter", v, err)
	}
	for _, bad := range []string{
		"TargetedByTarget$Valid You",             // a player spec: never an object target
		"TargetedByTarget$Valid Player.Opponent", // ditto
		"TargetedByTarget$Amount",                // only Valid is modelled
		"TargetedByTarget$Valid",                 // an empty spec fails closed
		"TargetedByTarget",                       // no property at all
	} {
		if v, err := lc.svarValue(bad, 0); err == nil {
			t.Errorf("%q lowered to %s, want a refusal", bad, v)
		}
	}
}

// TestTargetedByTargetPlayerSpecRefusesTheCard: the same gate through the whole
// card build -- rewriting the carrier's spec to a player one must refuse the
// card, not lower a reduction that can never fire.
func TestTargetedByTargetPlayerSpecRefusesTheCard(t *testing.T) {
	const n = "Not of This World"
	kp := kfhBuild(t, []string{n}, func(p *pool) {
		c := kfhClone(p.byName[n])
		c.Faces[0].SVars["CheckTgt"] = "TargetedByTarget$Valid You"
		p.byName[n] = c
	})
	kp.refusedWith(t, n, "player")
}

// expr returns the table's Expr a ValueRef names (a constant fails the test).
func (kp kfhPool) expr(t *testing.T, v kop.ValueRef) *kop.Expr {
	t.Helper()
	if v.K != kop.VExpr {
		t.Fatalf("ValueRef kind %v is not an expression", v.K)
		return nil
	}
	return &kp.pt.exprs[v.N]
}

// nestedTgtCount walks an expression tree (both Sub arms, recursively) for the
// nested-target count node; nil when the tree has none.
func (kp kfhPool) nestedTgtCount(t *testing.T, v kop.ValueRef) *kop.Expr {
	t.Helper()
	if v.K != kop.VExpr {
		return nil
	}
	e := &kp.pt.exprs[v.N]
	if e.Kind == kop.ExCountTgtEntry {
		return e
	}
	if a := kp.nestedTgtCount(t, e.Sub[0]); a != nil {
		return a
	}
	return kp.nestedTgtCount(t, e.Sub[1])
}

// arms reads an ExBranches node's constant arms.
func (kp kfhPool) arms(t *testing.T, e *kop.Expr) []int {
	t.Helper()
	var out []int
	for _, s := range [2]kop.ValueRef{e.Sub[0], e.Sub[1]} {
		if s.K != kop.VConst {
			t.Fatalf("a branch is %v, not a constant", s.K)
		}
		out = append(out, int(s.N))
	}
	return out
}
