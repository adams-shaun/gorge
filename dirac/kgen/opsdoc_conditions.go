package kgen

// opsConditions is the core OPS.md family for conditions. opsEntries (opsdoc.go)
// places it in the document; a new item goes in its own opsdoc_<mechanic>.go.
var opsConditions = []opsDoc{
	{"cond:Present", "CondPresent (F, Op, A)", "The number of battlefield objects matching F (relative to the controller) compares Op with A."},
	{"cond:Value", "CondValue (A, Op, B)", "A compares Op with B."},
	{"cond:Threshold", "CondThreshold", "Seven or more cards in the controller's graveyard."},
	{"cond:YourTurn", "CondYourTurn", "It is the controller's turn."},
	{"cond:RefMatches", "CondRefMatches (Ref, F, Op, A)", "The number of objects in Ref matching F compares Op with A (Scavenging Ooze: the exiled card was a creature; Soul-Shackled Zombie: a creature card among the exiled; Nine-Lives Familiar: it was cast by you)."},
	{"cond:And", "CondAnd (Sub[0], Sub[1])", "Both Sub[0] and Sub[1] hold, re-evaluated whenever the static is read (CR 611.3a)."},
	{"cond:Blessing", "CondBlessing", "The controller has the city's blessing (CR 702.131; gorge's static Condition$ Blessing, layers_static.go condBlessing): the one-way per-seat latch kernel kw_ascend.go keeps. Dusk Charger, Spire Winder."},
	{"cond:SourceZone", "CondSourceZone (Zone)", "The source object is in Zone now (gorge's IsPresent$ Card.StrictlySelf | PresentZone$, an intervening-if re-checked at resolution, CR 603.4)."},
	{"cond:RemMark", "CondRemMark", "The source's persistent remembered list is non-empty: the arena mask mechRem the resolving source's remember writes set (gorge's event-backed o.Remembered, the Choose \"remembered\" fold). It is the host half of a ConditionDefined$ Remembered group gate, whose group gorge reads as the source's persistent list UNIONED with the resolution's own memory (rememberedWithSource; Forge's SpellAbilityCondition.areMet reads getDefinedObjects(host, \"Remembered\"), which survives the resolution): the lowered gate is the kernel's own-set RefMatches OR this mark, so a chain that remembered nothing still meets the gate on an earlier resolution's write (Nezahal, Primal Tide's second discard-three activation, Aetherling's). The lowering ORs the mark in beside an unrestricted present filter only: a typed gate (Soul-Shackled Zombie's ConditionPresent$ Card.Creature) narrows the group to cards of the type, which the bare mark cannot see, and stays at today's per-resolution lowering. dirac/exprs.go condEval; the mark is set by every remember write on the resolving source (interp_gen.go remember)."},
}
