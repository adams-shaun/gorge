package effects

import "testing"

// TestBarePlayerCountBodyAppliesItsOpSuffix pins the Count$-less PlayerCount
// body with an /Op suffix (Avacyn's Judgment's
// SVar:MaxTgts:PlayerCountPlayers$Amount/Plus.MaxPermanents). The bare-body
// branch used to hand the whole string, suffix included, to the head
// dispatch, which matched nothing -- and NumResolved still reported the
// named SVar resolved at 0, so the spell's "any number of targets" bound
// forbade every target. An unmodelled operator stays unresolved, and
// NumResolvedStrict reports an unevaluated SVar body as unresolved.
func TestBarePlayerCountBodyAppliesItsOpSuffix(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	SetSVars(c, map[string]string{
		"MaxTgts":       "PlayerCountPlayers$Amount/Plus.MaxPermanents",
		"MaxPermanents": "Count$Valid Any",
		"Broken":        "PlayerCountPlayers$Amount/Unmodelled",
	})
	perms := EvalCount(h, c, "Count$Valid Any")
	if got, ok := EvalCountOK(h, c, "PlayerCountPlayers$Amount/Plus.MaxPermanents"); !ok || got != 2+perms {
		t.Fatalf("bare /Plus.<SVar> body = (%d,%v), want (%d,true)", got, ok, 2+perms)
	}
	if got, ok := EvalCountOK(h, c, "PlayerCountPlayers$Amount/Plus.3"); !ok || got != 5 {
		t.Fatalf("bare /Plus.3 body = (%d,%v), want (5,true)", got, ok)
	}
	if _, ok := EvalCountOK(h, c, "PlayerCountPlayers$Amount/Unmodelled"); ok {
		t.Fatal("unmodelled operator resolved")
	}
	s := sa(t, "SP$ DealDamage | ValidTgts$ Any | TargetMin$ 0 | TargetMax$ MaxTgts")
	if n, ok := NumResolvedStrict(h, c, s, "TargetMax", 1); !ok || n != 2+perms {
		t.Fatalf("NumResolvedStrict(MaxTgts) = (%d,%v), want (%d,true)", n, ok, 2+perms)
	}
	broken := sa(t, "SP$ DealDamage | ValidTgts$ Any | TargetMin$ 0 | TargetMax$ Broken")
	if n, ok := NumResolvedStrict(h, c, broken, "TargetMax", 1); ok || n != 1 {
		t.Fatalf("NumResolvedStrict(unmodelled body) = (%d,%v), want (1,false)", n, ok)
	}
}
