package dirac

// DebugReduceGatesNoFirst is the mutation switch: it
// stops thisTurnCastSpec hiding the in-flight spell's own push while
// buildCost composes its reductions, so an OnlyFirstSpell$ static (Radagast
// of Rhosgobel, Serah Farron) sees the cast it is pricing and the first
// covered spell pays full price. gorge's onlyFirstSpellUsed skips
// ev.Obj == id; the lockstep must diverge with this set. Test-only.
var DebugReduceGatesNoFirst bool
