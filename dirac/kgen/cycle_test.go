package kgen

import "testing"

// kcycleForms: the kcycle verification step. A type property beside a
// disjoint type head ("Land.Artifact", "Artifact.Creature") means BOTH
// types in gorge; the kernel's any-of Filter.Types cannot say that, and
// the old intersection lowered it to no type requirement at all, so
// kcycle refused it; typesall lowers the pair as the union in Types plus
// the conjunction in PredTypesAll (all=). A nested or same set still
// narrows.
var kcycleForms = []formCase{
	{card: "Sojourner's Companion", form: "TypeCycling:Land.Artifact (artifact landcycling): a hand search for artifact lands (PredTypesAll)", lir: "Search obj=You A=1 C=2 F={types=0x5 all=0x5}"},
	{card: "Steel Overseer", form: "Artifact.Creature+YouCtrl: your artifact creatures (PredTypesAll)", lir: "PutCounterAll A=1 CS=P1P1 F={types=0x6 all=0x6 ctrl=1}"},
	{card: "Kulrath Zealot", form: "TypeCycling:Basic (basic landcycling): a hand search for the basic supertype", lir: "Search obj=You A=1 C=2 F={types=0x200}"},
	{card: "Vedalken Aethermage", form: "TypeCycling:Wizard (wizardcycling): a hand search for the subtype", lir: "Search obj=You A=1 C=2 F={types=0x0 sub=Wizard}"},
}

func TestCycleForms(t *testing.T) { checkForms(t, kcycleForms) }
