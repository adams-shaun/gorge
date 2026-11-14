package kop

// ExtDmgRedirect (Nomads en-Kor, Warrior en-Kor; Gideon's Sacrifice): an
// Effect's DamageDone redirect replacement. Obj names the permanent the
// damage is dealt to instead (the ability's target, or the chosen card), C the
// kind:
//
//	DmgRedirectNext:  "the next 1 damage that would be dealt to CARDNAME this
//	  turn is dealt to target creature you control instead" (DB$
//	  ReplaceSplitDamage over Creature.EffectSource): the resolving source is
//	  the damaged host; the pool is spent by what is redirected, the record
//	  ends with the pool, with the host or the recipient leaving, and at the
//	  turn's end.
//	DmgRedirectScope: "all damage that would be dealt this turn to you and
//	  permanents you control is dealt to the chosen permanent instead" (DB$
//	  ReplaceEffect, VarName$ Affected over You,Permanent.YouCtrl): the
//	  resolving controller's, until the chosen permanent leaves or the turn ends.
//
// The redirected damage is a new event from the same source, so it meets
// protection, prevention and doubling again. dirac/repl_dmgredirect.go.
//
// The ordinal is 294, the highest free one below NumExtOps (the branch took 296
// before land gave it to ExtGrantKwAll; 295 is ExtTapOrUntapAll's).
const ExtDmgRedirect ExtOp = 293

// The C values of ExtDmgRedirect.
const (
	DmgRedirectNext = 1 + iota
	DmgRedirectScope
)

func init() { extOpNames[ExtDmgRedirect] = "DmgRedirect" }
