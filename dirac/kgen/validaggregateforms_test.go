package kgen

import "testing"

// validaggregateformsForms: lowerings, one carrier
// per class, checked like paramreadformsForms.
var validaggregateformsForms = []formCase{
	{card: "Shimmercreep", form: "Count$Valid Permanent.YouCtrl$Colors (ExValidAgg AggColors)", lir: "LoseLife obj=EachOpp A=ValidAgg({types=0x9f ctrl=1},agg=7)"},
	{card: "Abbot of Keral Keep", form: "Effect RememberObjects$ RememberedCard is the remembered set (gorge effectRememberedCodes)", lir: "1 MayPlay obj=Remembered B=1 C=5 CS=EOT"},
	{card: "Territorial Bruntar", form: "Effect MayPlay of Card.IsRemembered+nonLand (OpMayPlay B&MayPlayNonLand)", lir: "MayPlay obj=Remembered B=3 C=5 CS=EOT"},
	{card: "Silas Renn, Seeker Adept", form: "Effect MayPlay of a target's Card.IsRemembered+nonLand (OpMayPlay B&MayPlayNonLand)", lir: "MayPlay obj=Target0 B=2 C=3 CS=EOT"},
	{card: "Knacksaw Clique", form: "Effect MayPlay Duration$ EndOfTurn (gorge effectUntilEOTSet: this turn)", lir: "1 MayPlay obj=Remembered C=5 CS=EOT"},
	{card: "Aqueous Form", form: "static AddSVar$ of an AI-hint SVar:<Name>:<Value> grant (gorge GrantedSVar has no rules reader)", lir: "static Continuous F={types=0x2 EnchantedBy}"},
	{card: "Disrupting Scepter", form: "activation PlayerTurn$ True (gorge activationPhasesOK: the activator's turn only)", lir: "activate turnonce=false cond=YourTurn()"},
	{card: "Blink of an Eye", form: "node Condition$ Kicked of a spell (gorge conditionBareMet: the spell's FlagKicked)", lir: "1 SkipIf if=Kicked() skip=1\n     2 Draw obj=You A=1"},
	{card: "Fated Conflagration", form: "node ConditionPlayerTurn$ True (gorge conditions.go: the resolving controller's turn)", lir: "1 SkipIf if=YourTurn() skip=1\n     2 Scry obj=You A=2"},
}

func TestValidAggregateAndMayPlayForms(t *testing.T) { checkForms(t, validaggregateformsForms) }
