package dirac

// DebugHasCardsGE is the lockstep mutation of a
// kop.CmpGT compare reads as >= (Wojek Investigator's "more cards than you"
// becomes "at least as many"). Read cold in exprCompare and groupCountCmp
// (value_oppextreme.go).
var DebugHasCardsGE bool
