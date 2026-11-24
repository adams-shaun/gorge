package kgen

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerTapsformanaDynamic lowers Elvish Guidance's static TapsForMana
// trigger: ValidCard$ Card.AttachedBy, a DB$ Mana | Produced$ G | Amount$ X
// body with SVar:X:Count$Valid Elf. gorge's planner never folds a dynamic
// amount, so the enchanted land is deferred and the extra floats at the tap;
// the kernel models it as a StTapDynamic TapExtra whose Amount is the value
// reference, beside a class-only Taps line.
func TestLowerTapsformanaDynamic(t *testing.T) {
	var c *lCard
	for _, card := range lowerNamed(t, []string{"Elvish Guidance"}) {
		if card.Name == "Elvish Guidance" {
			c = card
		}
	}
	if c == nil {
		t.Fatal("Elvish Guidance: not lowered")
	}
	if len(c.Refusals) != 0 {
		t.Fatalf("Elvish Guidance refused: %v", c.Refusals)
	}
	var st *lStatic
	for i := range c.Statics {
		if c.Statics[i].Kind == "TapExtra" {
			st = &c.Statics[i]
			break
		}
	}
	if st == nil {
		t.Fatal("Elvish Guidance: no TapExtra static")
	}
	for _, fl := range []string{"TapAnyPlayer", "TapDynamic"} {
		if !slices.Contains(st.Flags, fl) {
			t.Errorf("Elvish Guidance: TapExtra flags %v, want %s", st.Flags, fl)
		}
	}
	if st.TapCol != 4 {
		t.Errorf("Elvish Guidance: TapExtra colour %d, want green (4)", st.TapCol)
	}
	if st.Amount.Kind == vNone {
		t.Error("Elvish Guidance: TapExtra without its dynamic amount")
	}
	if st.F == nil {
		t.Error("Elvish Guidance: TapExtra without its attached filter")
	}
	lines := 0
	for _, ab := range c.Abilities {
		tr := ab.Trigger
		if tr == nil || !tr.TapsForMana {
			continue
		}
		lines++
		if tr.Event != "Taps" || tr.TapPlan != uint8(kop.TapPlanDefer) || tr.Whose != whoAny {
			t.Errorf("Elvish Guidance: Taps line event %q plan %d whose %d, want Taps/Defer/any", tr.Event, tr.TapPlan, tr.Whose)
		}
	}
	if lines != 1 {
		t.Errorf("Elvish Guidance: %d class-only Taps lines, want 1", lines)
	}
}
