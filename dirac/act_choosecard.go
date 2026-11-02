package dirac

// DebugChooseCardAmount2 (TestKchoosecardparamsMutation): a mandatory
// ChooseCard's pick count is one. kgen reads Amount$ 1 only to prove it is
// the default; the instruction carries no count operand to perturb. The
// mutation therefore perturbs the chooser instead: the opponent answers the
// ask, where gorge asks the resolving controller (chooseCardChoosers). It
// must make the lockstep diverge; false everywhere else.
var DebugChooseCardAmount2 bool
