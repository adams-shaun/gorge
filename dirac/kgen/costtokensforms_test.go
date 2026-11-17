package kgen

import (
	"strings"
	"testing"
)

// The cost tokens `costHand` had no arm for, named by a real corpus carrier
// each: Vexing Devil's literal DamageYou<4> unless cost, Abandon Hope's
// announced-X Discard<X/Card/card> beside a printed {X} and Channeled Force's
// with none (the hand cards are the bound). Academy Loremaster's
// Draw<1/Player.TriggeredPlayer> is checked below: the Draw instruction
// lowers, but the card still refuses its RaiseCost Effect static (a separate
// gap), so checkForms' no-refusal arm cannot name it.
var costTokenForms = []formCase{
	{card: "Vexing Devil", form: "UnlessCost$ DamageYou<4> (a literal amount)", lir: "CS=DamageYou"},
	{card: "Abandon Hope", form: "Cost$ Discard<X/Card/card> (the announced X)", lir: "DiscardX"},
	// Discard<X/Spec> with no printed {X}: the announced part is the only X
	// the cost carries, so it is the bound (gorge cast_subcounter.go xAsk).
	{card: "Channeled Force", form: "Cost$ Discard<X/Card/card> with no printed {X}", lir: "DiscardX"},
}

func TestCostTokensForms(t *testing.T) {
	checkForms(t, costTokenForms)

	// Academy Loremaster: the unless cost's Draw<1/Player.TriggeredPlayer>
	// lowers to the pay instruction (the triggered player draws).
	cs := lowerNamed(t, []string{"Academy Loremaster"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards", len(cs))
	}
	var sb strings.Builder
	dumpLIR(&sb, cs[0])
	if strings.Contains(sb.String(), "cost token") {
		t.Errorf("Academy Loremaster: the Draw cost token still refuses:\n%s", sb.String())
	}
	if !strings.Contains(sb.String(), "CS=Draw") {
		t.Errorf("Academy Loremaster: lowered IR lacks the Draw pay instruction:\n%s", sb.String())
	}
}
