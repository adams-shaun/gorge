package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOppEnteredCondValue: PlayerCountOpponents$ConditionGE<n>
// ThisTurnEntered_<spec> lowers to the per-opponent entered count (the
// ordinary ThisTurnEntered head, read from each opponent's side) against n;
// the SpellsCastThisTurn property still lowers as before and every other shape
// refuses.
func TestOppEnteredCondValue(t *testing.T) {
	lc := &lowerCtx{}
	v, err := lc.svarValue("PlayerCountOpponents$ConditionGE2 ThisTurnEntered_Battlefield_Creature.YouCtrl", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := kop.OppExtCountCmp | kop.OppExtMemberExpr
	if v.Kind != vOppExtreme || v.N != want || v.Op != opGE || len(v.Sub) != 2 {
		t.Fatalf("got kind %v N %#x op %v subs %d; want OppExtreme N %#x GE with 2 operands", v.Kind, v.N, v.Op, len(v.Sub), want)
	}
	if !v.Sub[0].isConst() || v.Sub[0].N != 2 {
		t.Errorf("bound %+v, want the constant 2", v.Sub[0])
	}
	if f := v.Sub[1]; f.Kind != vThisTurnEntered || f.Zone != uint8(kop.ZoneBattlefield) || f.F == nil {
		t.Errorf("member figure %+v, want a Battlefield ThisTurnEntered head with a filter", f)
	}
	s, err := lc.svarValue("PlayerCountOpponents$ConditionGE3 SpellsCastThisTurn", 0)
	if err != nil || s.Kind != vOppExtreme || s.N != kop.OppExtCountCmp|kop.OppExtSpellsCast {
		t.Errorf("SpellsCastThisTurn: got kind %v N %#x, %v", s.Kind, s.N, err)
	}
	for _, body := range []string{
		"PlayerCountOpponents$ConditionLT2 ThisTurnEntered_Battlefield_Creature.YouCtrl",
		"PlayerCountOpponents$ConditionGE2 ThisTurnEntered_Battlefield_Creature.YouCtrl$CardPower",
		"PlayerCountOpponents$ConditionGE2 ThisTurnEntered_Stack_Card",
		"PlayerCountOpponents$ConditionGE2 ThisTurnEntered_",
		"PlayerCountOpponents$ConditionGE0 ThisTurnEntered_Battlefield_Creature.YouCtrl",
		"PlayerCountOpponents$ConditionGE2 CardsDrawn",
	} {
		if _, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%s lowered, want a refusal", body)
		} else if !strings.Contains(err.Error(), "PlayerCountOpponents") && !strings.Contains(err.Error(), "ThisTurnEntered") {
			t.Errorf("%s: refusal %q names neither the head nor ThisTurnEntered", body, err)
		}
	}
}
