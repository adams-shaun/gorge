package forge

// ValueKeys are the parameters whose value is a number or an expression,
// sorted. A key belongs when over 95% of its corpus uses are an integer, X,
// an expression, or a name resolving to a numeric body. Out: ChangeNum (77%,
// "All"), ChoiceAmount (90%, "Any"), VarValue (81%, referents), WithTotalCMC
// (93%), and ReduceCost, a mana cost ("1 U") that is numeric only when generic.
var ValueKeys = []string{
	"ActivationLimit", "Adapt", "AddPower", "AddToughness", "Amount", "Annihilator", "Bolster", "CharmNum",
	"CheckSVar", "ChoiceNum", "ConniveNum", "CounterMapValues", "CounterNum", "CounterNum2", "DigNum",
	"DividedAsYouChoose", "Each", "FaceDownPower", "FaceDownToughness", "GameActivationLimit", "LibraryPosition",
	"LibraryPosition2", "LifeAmount", "Max", "MaxRepeat", "MaxRevealed", "Min", "MinAmount", "MinCharmNum",
	"MinChoiceAmount", "MinTotalCMC", "Modifier", "Monstrosity", "Multiplier", "NoteNumber", "Num", "NumAtt",
	"NumCards", "NumCopies", "NumDef", "NumDmg", "NumRandomChoices", "NumTurns", "PeekAmount", "Power", "Renown",
	"RepeatNum", "RevealNumber", "ScryNum", "SetPower", "SetToughness", "Sides", "Support", "TargetMax",
	"TargetMin", "Times", "TokenAmount", "TokenPower", "TokenToughness", "Toughness", "WithCountersAmount",
}
