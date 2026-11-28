package forge

// ValueKeys are the parameters whose value is a number or an expression,
// sorted. A key belongs when over 95% of its corpus uses are an integer, X,
// an expression, or a name resolving to a numeric body (ChangeNum, at 77%
// because of "All", does not).
var ValueKeys = []string{"Adapt", "AddPower", "AddToughness", "Amount", "Annihilator", "Bolster", "CounterNum",
	"DigNum", "DividedAsYouChoose", "LifeAmount", "Max", "Min", "MinAmount", "Num", "NumAtt", "NumCards", "NumDef",
	"NumDmg", "NumTurns", "PeekAmount", "Power", "Renown", "ScryNum", "SetPower", "SetToughness", "TargetMax",
	"TargetMin", "TokenAmount", "TokenPower", "TokenToughness", "Toughness", "WithCountersAmount"}
