package kgen

import "testing"

// kentercastForms: the entering spell's cast record in
// a self enters replacement's spend read and in wasCastFromYourGraveyard.
var kentercastForms = []formCase{
	{card: "Freestrider Commando", form: "etbCounter CheckSVar$ X (Count$CastTotalManaSpent) EQ0: the entering spell's spend", lir: "repl EntersWith F=nil cond=Value(op3,CastManaSpent,0)\n     0 EnterWithCounters obj=Replaced A=2 CS=P1P1"},
	{card: "Rocket-Powered Goblin Glider", form: "ChangesZone ValidCard$ Card.Self+wasCastFromYourGraveyard (PredWasCastEnter PredN 3)", lir: "trigger ETB F={types=0x0 Self cmp(29 0 3  )}"},
	{card: "Doc Aurlock, Grizzled Genius", form: "wasCastFromYourGraveyard in a ReduceCost static stays refused", refused: "wasCast outside a battlefield-entering trigger's ValidCard$"},
	{card: "Dyadrine, Synthesis Amalgam", form: "RemoveCounter Choices$ ChoiceNum$ 2 lowers to OpSupport's remove form", lir: "0 Support A=2 C=14 CS=P1P1"},
}

func TestEntercastLowers(t *testing.T) { checkForms(t, kentercastForms) }
