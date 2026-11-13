package kgen

import "testing"

// Outmuscle's PutCounter carries AIManaPref$ (Forge's AI mana hint, read
// nowhere in gorge): it is no refusal on a PutCounter, and stays one on an
// Untap (Basalt Monolith diverges from gorge on which permanents pay).
func TestAIManaPrefPutCounterOnly(t *testing.T) {
	put := "Name:Test Bear\nManaCost:2\nTypes:Creature Bear\nPT:1/1\nA:AB$ PutCounter | Cost$ 1 | CounterType$ P1P1 | CounterNum$ 1 | AIManaPref$ G\nOracle:x\n"
	if r := lowerScript(t, "Test Bear", put); len(r) != 0 {
		t.Fatalf("PutCounter with AIManaPref$ refuses: %v", r)
	}
	untap := "Name:Test Rock\nManaCost:2\nTypes:Artifact\nA:AB$ Untap | Cost$ 3 | AIManaPref$ NotSameCard\nOracle:x\n"
	if r := lowerScript(t, "Test Rock", untap); len(r) == 0 {
		t.Fatal("Untap with AIManaPref$ lowers; its payment diverges from gorge, it must stay refused")
	}
}
