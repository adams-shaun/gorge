package kgen

import "testing"

// TestLowerRepeatCheckSVar: Kroxa, Titan of Death's Hunger's RepeatEach body
// gate (ConditionCheckSVar$ X beside ConditionSVarCompare$ EQ0, with the
// gate SVar X:Remembered$Valid Card.nonLand+RememberedPlayerOwn) lowers as the
// parameterless vRemEachOwnNonLand read behind a SkipIf: RepeatEach over the
// opponents, then SkipIf (new value, EQ 0), then the LoseLife over RefEach,
// then the ClearRemembered. Both trigger nodes lower the same body.
func TestLowerRepeatCheckSVar(t *testing.T) {
	c := repeatEachTargetedCard(t, "Kroxa, Titan of Death's Hunger")
	for _, rf := range c.Refusals {
		t.Errorf("Kroxa refused: %s", rf.Reason)
	}
	sawRepeat, sawRead := false, false
	for _, a := range c.Abilities {
		for i, in := range a.Prog {
			if in.Op != "RepeatEach" || in.Obj.Kind != rEachOpp || in.C&1 == 0 {
				continue
			}
			sawRepeat = true
			if in.Skip != 2 || i+3 >= len(a.Prog) {
				t.Fatalf("Kroxa RepeatEach skip %d, want the gate SkipIf and the LoseLife", in.Skip)
			}
			skip, lose, cr := a.Prog[i+1], a.Prog[i+2], a.Prog[i+3]
			if skip.Op != "SkipIf" || skip.Cond.Kind != cValue || skip.Cond.A.Kind != vRemEachOwnNonLand ||
				skip.Cond.Op != opEQ || !skip.Cond.B.isConst() || skip.Cond.B.N != 0 {
				t.Errorf("Kroxa gate %s, want SkipIf RemEachOwnNonLand EQ0", skip.String())
			} else {
				sawRead = true
			}
			if lose.Op != "LoseLife" || lose.Obj.Kind != rEach || lose.A.Kind != vConst || lose.A.N != 3 {
				t.Errorf("Kroxa body op %s, want LoseLife 3 over RefEach", lose.String())
			}
			if cr.Op != "ClearRemembered" {
				t.Errorf("Kroxa cleanup %s, want ClearRemembered", cr.String())
			}
		}
	}
	if !sawRepeat || !sawRead {
		t.Fatalf("Kroxa lowered RepeatEach %v with gate %v, want both", sawRepeat, sawRead)
	}
}
