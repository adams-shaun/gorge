package dirac

// DebugExpelGainTarget1 (TestKickerSplitTargetLifeGainMutation) restores the
// pre-fix lowering of Expel the Unworthy's trailing gain: one GainLife reading
// target slot 1 (the kicked sub's slot), which is absent in an unkicked game.
// kgen reads it while lowering, so the mutation test rebuilds the card's
// table with the flag set.
var DebugExpelGainTarget1 bool
