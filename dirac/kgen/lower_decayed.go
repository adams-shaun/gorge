package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// Decayed (CR 702.147a, Tainted Adversary's token). gorge reads it as
// the hidden keyword "can't block" (cards/hiddenkeyword.go) plus a synthesized
// Attacks trigger (rules/trigger_match.go Decayed) whose body is the builtin
// __kwDecayedAttack: a one-shot DelayedTrigger Phase$ EndCombat that sacrifices
// the decayed creature itself. kgen lowers exactly that: KwCantBlock and the
// same Attacks trigger over the existing end-of-combat delayed program.
// OncePerSource$ only stops a second resolution arming a second sacrifice; a
// second sacrifice of the same (already gone) creature is a no-op, so the
// kernel need not read it. A granted Decayed stays refused (the grant paths).

const decayedAttackSVar = "__kwDecayedAttack"

// decayedRegistration reports whether ps is the builtin Decayed registration.
func decayedRegistration(ps *params) bool {
	return ps.str("Execute") == "__kwDecayedSacrifice" && ps.str("OncePerSource") == "True" &&
		ps.str("RememberChain") == "False" && ps.str("Phase") == "EndCombat"
}

// lowerDecayed lowers K:Decayed.
func (lc *lowerCtx) lowerDecayed() {
	lc.out.Kw = append(lc.out.Kw, "CantBlock")
	t := cards.Trigger{Mode: "Attacks", Params: map[string]string{
		"Mode": "Attacks", "ValidCard": "Card.Self", "Execute": decayedAttackSVar,
		"TriggerDescription": "When this creature attacks, sacrifice it at end of combat.",
	}, Effect: cards.ResolveSVar(lc.face.SVars, decayedAttackSVar)}
	lc.guard("K:Decayed", func() error { return lc.lowerTriggered(t, "") })
}
