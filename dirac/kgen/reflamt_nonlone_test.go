package kgen

import (
	"strings"
	"testing"
)

// reflamtNonLoneScript is Incubation Druid's reflected Amount$ read over a
// land you control, alone or beside the plain {T}: Add {C} first ability.
func reflamtNonLoneScript(beside bool) string {
	var b strings.Builder
	b.WriteString("Name:ReflAmtProbe\nManaCost:no cost\nTypes:Land Gate\n")
	if beside {
		b.WriteString("A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 1 | SpellDescription$ Add a colourless mana.\n")
	}
	b.WriteString("A:AB$ ManaReflected | Cost$ T | ColorOrType$ Type | Valid$ Land.YouCtrl | Amount$ ProbeAmt | ReflectProperty$ Produce | SpellDescription$ Add X mana of any type a land you control could produce.\n")
	b.WriteString("SVar:N:Count$Valid Card.Self+counters_GE1_P1P1\n")
	b.WriteString("SVar:ProbeAmt:Count$Compare N GE1.3.1\n")
	return b.String()
}

// TestReflAmtNonLoneRefusal: Incubation Druid's reflected Amount$ read lowers
// alone (the kernel's reflAmt census gap, mana_reflamt.go), but beside another
// mana ability it refuses: the gap registers position-independently (gen.go
// registersGapOther) while the window enabler is the first mana ability's
// alone (gen.go j == firstManaAb), so the gap would sit registered yet never
// offered — gorge's manual window lists every untapped source.
func TestReflAmtNonLoneRefusal(t *testing.T) {
	lone := lowerScript(t, "ReflAmtProbe", reflamtNonLoneScript(false))
	if len(lone) != 0 {
		t.Errorf("lone reflected Amount$ read: want no refusal, got %q", strings.Join(lone, "; "))
	}
	beside := lowerScript(t, "ReflAmtProbe", reflamtNonLoneScript(true))
	if len(beside) == 0 {
		t.Fatal("reflected Amount$ read beside {T}: Add {C}: want a refusal, got none")
	}
	all := strings.Join(beside, "; ")
	if !strings.Contains(all, "reflected Amount$ mana ability beside another mana ability") {
		t.Errorf("want the non-lone Amount$ refusal, got %q", all)
	}
}

// TestReflAmtLifeCostRefusal: a lone reflected Amount$ ability with a
// PayLife<N> cost beside {T} refuses: the kernel's gap discriminator wants {T}
// alone, so it would lower as the one-mana life imprint view while gorge
// defers the amount.
func TestReflAmtLifeCostRefusal(t *testing.T) {
	script := strings.Replace(reflamtNonLoneScript(false), "Cost$ T |", "Cost$ T PayLife<1> |", 1)
	if script == reflamtNonLoneScript(false) {
		t.Fatal("precondition: the script's cost was not rewritten")
	}
	got := strings.Join(lowerScript(t, "ReflAmtProbe", script), "; ")
	if !strings.Contains(got, "lone reflected Amount$ mana ability with a life cost") {
		t.Errorf("want the life-cost Amount$ refusal, got %q", got)
	}
}
