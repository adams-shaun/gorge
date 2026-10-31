package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A planned cast's witness prices a Phyrexian pip as neither mana nor life, and the
// answer settles it (CR 107.4f). Paying the pip with mana outdates the witness
// before any step ran, and the cast is still paid without the manual window.
func TestPlannedCastPaysAPhyrexianPipWithMana(t *testing.T) {
	t.Parallel()
	const (
		imp   = "Name:Test Imp\nManaCost:2 BP\nTypes:Creature Imp\nPT:1/1\nOracle:x\n"
		swamp = "Name:Test Swamp\nTypes:Land Swamp\nA:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Black.\nOracle:x\n"
	)
	for _, tc := range []struct {
		kind string
		taps int
	}{{"pay_B", 3}, {"pay_life", 2}} {
		e, _, spell := newFixtureDeck(t, 9700, imp)
		toMain1(t, e)
		e.paidManaCensus = true
		var lands []state.ObjID
		for range 3 {
			lands = append(lands, onBoard(t, e, 0, swamp))
		}
		d := ppPriority(t, e)
		e.EnsurePaymentActions()
		ppSubmitPlan(t, e, d, ppAction(t, e.Pending(), spell))
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose {
			t.Fatalf("%s: pending %+v, want the pip ask", tc.kind, ask)
		}
		idx := -1
		for _, o := range ask.Options {
			if o.Kind == tc.kind {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%s: not offered: %+v", tc.kind, ask.Options)
		}
		submitChoices(t, e, idx)
		if d := e.Pending(); d == nil || d.PaymentFallback != nil || d.Kind != decision.KPriority {
			t.Fatalf("%s: pending %+v, want the spell paid without a fallback", tc.kind, d)
		}
		tapped := 0
		for _, id := range lands {
			if e.G.Obj(id).Tapped {
				tapped++
			}
		}
		if tapped != tc.taps || len(e.G.Stack) != 1 {
			t.Errorf("%s: %d lands tapped, stack %v; want %d and the spell", tc.kind, tapped, e.G.Stack, tc.taps)
		}
	}
}
