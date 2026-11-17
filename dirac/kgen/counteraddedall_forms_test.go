package kgen

import "testing"

// counterAddedAllForms: Mode$ CounterAddedAll, the batch watcher over other
// permanents (gorge trigmatch counterAddedAllMatches), lowered onto the
// kernel's once-per-batch TrigCounterAddedAll event. Invisible Woman is the
// ValidSource$ You shape, Bioessence Hydra the amount-reading planeswalker
// line, Cloaked Cadet the ActivationLimit$ 1 once-per-turn latch.
var counterAddedAllForms = []formCase{
	{card: "Invisible Woman, Sue Storm", form: "CounterAddedAll P1P1 on other Heroes, ValidSource$ You", lir: "trigger CounterAddedAll"},
	{card: "Bioessence Hydra", form: "CounterAddedAll LOYALTY on planeswalkers you control", lir: "trigger CounterAddedAll"},
	{card: "Cloaked Cadet", form: "CounterAddedAll P1P1 on Humans you control, ActivationLimit$ 1", lir: "trigger CounterAddedAll"},
}

func TestCounterAddedAllForms(t *testing.T) { checkForms(t, counterAddedAllForms) }
