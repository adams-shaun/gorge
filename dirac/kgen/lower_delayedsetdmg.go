package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// digSetDelayedDamage reports whether a triggered chain that dug — and
// remembered — several cards registers an end-step delayed trigger over that
// whole set whose body counts the ones still exiled: Dragonhawk, Fate's
// Tempest's shape, RememberObjects$ Remembered beside a Dig's
// RememberChanged$ set (the exiled top X cards), the Execute$ body exactly
//
//	DamageAll | ValidPlayers$ Player.Opponent | NumDmg$ Y
//	Y:Count$ValidExile Card.IsTriggerRemembered/Twice
//
// with one Cleanup ClearRemembered$ sub-ability after it (the second
// registration the same turn must not re-remember the first dig's cards, and
// the count reads what THIS trigger remembered). Anything else keeps the
// delayedRemember refusal: the record's DpMany program (dirac/
// trig_delayedmany.go) carries the set only for bodies the count read covers.
//
// The lowering is not complete for every game: the DpMany program holds at
// most four objects, so a dig of five or more cards (Dragonhawk's X counts
// the creatures with power 4 or greater, which can pass four) stalls the
// kernel at the registration (StallDProg, a declared capacity stall, not a
// wrong answer). The kernel also counts an object only while it is the one the
// program remembered (CR 400.7): gorge's IsTriggerRemembered matches the
// ObjID alone, so a dug card played from exile and exiled again before the
// end step counts in gorge and not here (a gorge ticket, not a kernel
// approximation).
func (lc *lowerCtx) digSetDelayedDamage(exec *cards.SA) bool {
	if lc.ab == nil || lc.ab.Kind != akTriggered || !lc.hasRememberWriter() ||
		lc.rememberedHoldsOne() || !lc.rememberedFromDig() {
		return false
	}
	if exec == nil || exec.API != "DamageAll" || len(exec.Params) != 2 ||
		strings.TrimSpace(exec.Params["ValidPlayers"]) != "Player.Opponent" {
		return false
	}
	y, ok := exec.Params["NumDmg"]
	if !ok || lc.face.SVars[strings.TrimSpace(y)] != "Count$ValidExile Card.IsTriggerRemembered/Twice" {
		return false
	}
	// the Cleanup ClearRemembered$ is the delayed trigger's own SubAbility$
	// (the chain's next node, DBCleanup): the body's Execute$ names only the
	// DamageAll
	c := lc.curNode.Sub
	return c != nil && c.API == "Cleanup" && c.Sub == nil &&
		c.Params["ClearRemembered"] == "True" && len(c.Params) == 1
}

// delayedBody lowers an end-step delayed trigger's Execute$ chain into lat and
// completes the registration instruction in. For a dig's whole remembered set
// (delayedRemember set lc.delayedSetDmg) it marks the latent so the body's
// still-exiled count reads the carried set, checks the body lowered to the one
// DamageAll, and selects the kernel's many-object program (dirac/
// trig_delayedmany.go).
func (lc *lowerCtx) delayedBody(lat *lAbility, exec *cards.SA, in *lInstr) error {
	setDmg := lc.delayedSetDmg
	lc.delayedSetDmg = false
	lat.DelayedSetDmg = setDmg
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	if !setDmg {
		return nil
	}
	if len(lat.Prog) != 1 || lat.Prog[0].Op != "DamageAll" {
		return refuse("delayed dig-set body lowered to %d ops, want one DamageAll over the still-exiled remembered count", len(lat.Prog))
	}
	in.B = cst(3)
	return nil
}
