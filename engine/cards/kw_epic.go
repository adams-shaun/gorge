// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwEpic expands K:Epic (CR 702.50a, Enduring Ideal / Eternal Dominion) into
// the arming instruction its two clauses need.
//
// "For the rest of the game, you can't cast spells" is an Effect-delivered
// CantBeCast static (the Bilbo's Gambit route: engine/rules/statics_cast.go's
// castRestrictionSources reads registered CantBeCast continuous effects beside
// the printed-S: route). Duration$ Permanent is the game-lasting lifetime.
//
// "At the beginning of each of your upkeeps, copy this spell except for its
// epic ability" is a self-re-arming one-shot DelayedTrigger: API$
// DelayedTrigger registrations fire once, so the upkeep body re-arms the
// identical registration each firing (self-sustaining recurrence without a
// new primitive). NextTurn$ True is load-bearing on BOTH registrations: the
// delayed registry's phase gate accepts the current step, so a re-arm made
// during the upkeep itself would re-fire inside that same upkeep forever;
// MinTurn=Turn+1 pins each firing to the NEXT turn's upkeep, which is also
// the CR reading for a spell that resolved after its upkeep began.
//
// The arm is the resolution-tail instruction (the Cipher shape,
// engine/rules/epic.go's epicArmTailFor splicing __kwEpicArm into the
// resolving spell's own chain), NOT a trigger: CR 702.50a grants both clauses
// to a spell that RESOLVED, and the stack exit is a poor discriminator for
// that -- the engine's resolution tail, a counter/fizzle/reversal move and
// every effect-driven stack exit (Bilbo's Gambit's "Return target spell to
// its owner's hand") all carry empty Text on the move event alike, so a
// ChangesZone arming trigger over-fires on a bounced Epic. Riding the
// resolution chain, a countered, fizzled or bounced Epic never reaches the
// arm. The !copiedSpell half of the old gate is the splice site's
// `!o.IsCopy` guard in engine/rules/stack.go: the upkeep body mints stack
// copies, and a copy's own resolution -- the copy is made except for its
// epic ability (CR 702.50a) -- must re-arm nothing.
//
// The upkeep body copies the source (Ctx.Source is the registration's Source,
// CR 603.7d: the Epic card itself, wherever it now sits) as a spell SNAPSHOT
// -- a resolved card spell keeps its compiled Ability across the stack exit
// -- which engine/effects/copy.go's Defined$ Self arm mints through the
// StackCopy event. MayChooseTarget$ True gives the copy the CR 707.10c
// new-targets election (Eternal Dominion's "You may choose a new target for
// the copy").
func kwEpic(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.setSVar("__kwEpicArm",
		"DB$ Effect | StaticAbilities$ __kwEpicCantCast | Duration$ Permanent | SubAbility$ __kwEpicDelay")
	f.setSVar("__kwEpicCantCast",
		"Mode$ CantBeCast | ValidCard$ Card | Caster$ You | Description$ For the rest of the game, you can't cast spells.")
	f.setSVar("__kwEpicDelay",
		"DB$ DelayedTrigger | Mode$ Phase | Phase$ Upkeep | NextTurn$ True | ValidPlayer$ You | Execute$ __kwEpicUpkeep | RememberChain$ False")
	f.setSVar("__kwEpicUpkeep",
		"DB$ CopySpellAbility | Defined$ Self | MayChooseTarget$ True | SubAbility$ __kwEpicDelay")
}

func init() { registerKeyword(kwEpic, "Epic") }
