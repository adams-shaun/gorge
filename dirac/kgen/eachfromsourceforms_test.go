package kgen

import "testing"

// eachFromSourceForms: the eachfromsource lifts (kgen follow-ons to gorge,
// and a1ab5c307 at pin 81dd3f940), one carrier per accepted form and one
// per form left refused, checked like filterpredicateformsForms.
var eachFromSourceForms = []formCase{
	// SacrificeAll Controller$ You narrows Defined$ Self to the
	// resolving controller's permanent (gorge controlledOK)
	{card: "Fog Elemental", form: "SacrificeAll Defined$ Self Controller$ You (the Self clause takes ctrl=You)", lir: "SacrificeAll F={types=0x0 ctrl=1 Self}"},
	// the modified property (CR 700.9) is PredModified
	{card: "Orochi Merge-Keeper", form: "modified (PredModified) in a static's Card.Self filter", lir: "Self cmp(14 0 0  )"},
	{card: "Kami's Flare", form: "modified in a ConditionPresent$ filter", lir: "SkipIf if=Present({types=0x2 ctrl=1 cmp(14 0 0  )},op1,1) skip=1"},
	// a1ab5c307: CounterType$ EachFromSource is OpPutCountersFrom
	{card: "Star Pupil", form: "PutCounter CounterType$ EachFromSource EachFromSource$ TriggeredCardLKICopy (OpPutCountersFrom, the trigger object)", lir: "PutCountersFrom obj=Target0 A=1 obj2=Triggered"},
	{card: "Graft Surgeon", form: "EachFromSource onto an up-to-one target", lir: "PutCountersFrom obj=Target0 A=1 obj2=Triggered"},
	{card: "Zack Fair", form: "EachFromSource$ Self (the cost-sacrificed source's snapshot); its Attach Choices$ pick lowers since ", lir: "PutCountersFrom obj=Target0 A=1 obj2=Self"},
	// RemoveCounter CounterType$ All CounterNum$ All is OpClearCounters
	{card: "Vampire Hexmage", form: "RemoveCounter CounterType$ All CounterNum$ All on a target permanent (OpClearCounters)", lir: "ClearCounters obj=Target0"},
	{card: "Enchanted River's Grasp", form: "RemoveCounter All/All Defined$ Enchanted", lir: "ClearCounters obj=Equipped"},
	{card: "Final Act", form: "RemoveCounter All/All Defined$ Opponent: a player subject, kept refused", refused: "RemoveCounter All of every kind on a player"},
	{card: "Mutated Cultist", form: "RemoveCounter All/All over a Permanent,Opponent target slot (and RememberAmount$, which gorge leaves loud): kept refused", refused: "RemoveCounter All of every kind on a player (Target0)"},
	// api RemoveCounterAll is OpRemoveCounterAll
	{card: "Heartmender", form: "RemoveCounterAll CounterNum$ 1 of M1M1 from each creature you control", lir: "RemoveCounterAll A=1 CS=M1M1 F={types=0x2 ctrl=1}"},
	{card: "Inhumaniac", form: "RemoveCounterAll AllCounters$ True (B=1: every counter of the kind)", lir: "RemoveCounterAll A=1 B=1 CS=P1P1 F={types=0x0 Self} // all of the kind"},
	{card: "Aether Snap", form: "RemoveCounterAll AllCounterTypes$ True without CounterType$: gorge effRemoveCounterAll returns without acting, kept refused", refused: "RemoveCounterAll without CounterType$"},
	{card: "Dust of Moments", form: "RemoveCounterAll ValidZone$ Exile over suspended cards: kept refused", refused: "suspended"},
	{card: "Hold for Ransom", form: "SacrificeAll Controller$ OriginalHostController (the corpus's one non-You Defined$ carrier): kept refused, on its Defined$ OriginalHost first", refused: `SacrificeAll without ValidCards$ (Defined$ "OriginalHost")`},
}

func TestEachFromSourceAndRemoveCounterForms(t *testing.T) { checkForms(t, eachFromSourceForms) }
