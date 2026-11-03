package dirac

// DebugReduceGatesNoFirst is the mutation switch: it
// stops thisTurnCastSpec hiding the in-flight spell's own push while
// buildCost composes its reductions, so an OnlyFirstSpell$ static (Radagast
// of Rhosgobel, Serah Farron) sees the cast it is pricing and the first
// covered spell pays full price. gorge prices before the push, so no push of the priced object
// is dropped but the in-flight one; the lockstep must diverge with this set. Test-only.
var DebugReduceGatesNoFirst bool
