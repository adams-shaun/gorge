package kgen

import "testing"

// adamantSitesForms: Count$Adamant read off the cast's stack-entry spend at a
// spell's own resolution and at the card's own ETB replacement (lower_evoke.go
// adamantValue), beside the self-ETB trigger header it already lowered at.
var adamantSitesForms = []formCase{
	{card: "Cauldron's Gift", form: "spell's own ConditionCheckSVar$ Count$Adamant.Black.1.0", lir: "SkipIf if=Value(op1,Adamant(n=26,1,0),1)"},
	{card: "Foreboding Fruit", form: "spell sub-ability TokenAmount$ Count$Adamant.Black.1.0", lir: "CreateToken obj=You A=Adamant(n=26,1,0)"},
	{card: "Ardenvale Paladin", form: "K:etbCounter Adamant$ White (self ETB replacement CheckSVar$)", lir: "repl EntersWith F=nil cond=Value(op1,Adamant(n=24,1,0),1)"},
	{card: "Garenbrig Paladin", form: "K:etbCounter Adamant$ Green (self ETB replacement CheckSVar$)", lir: "repl EntersWith F=nil cond=Value(op1,Adamant(n=28,1,0),1)"},
	{card: "Sundering Stroke", form: "threshold 7 past the saturating spend count", refused: "threshold 7"},
	{card: "Henge Walker", form: "Adamant$ Any (no single colour)", refused: `colour "Any"`},
}

func TestAdamantSitesForms(t *testing.T) { checkForms(t, adamantSitesForms) }
