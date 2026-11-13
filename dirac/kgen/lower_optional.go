package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// mayGate reads an effect's Optional$ True ("you may <effect>"): gorge
// asks the resolving controller yes / no before the effect (effMill's
// mill_optional, effAttach's attach_optional), which lowers to an OpMayAsk
// over the one instruction the caller emits next. guard is the kop.MayGuard*
// under which gorge asks. Any value but True refuses, and so does a
// subject other than the controller (gorge asks each affected player).
func (lc *lowerCtx) mayGate(ps *params, guard int, who lRef) error {
	v, ok := ps.get("Optional")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "True" {
		return refuse("Optional$ %q", v)
	}
	if (guard == kop.MayGuardMill || guard == kop.MayGuardLibraryTop) && who.Kind != rYou {
		return refuse("Optional$ on another player's effect")
	}
	lc.emit(lInstr{Op: "MayAsk", C: guard, Skip: 1, Note: "you may"})
	return nil
}

// an effect's OptionalDecider$ on Draw and DealDamage ("you
// may draw N", "that creature's controller may have it deal 1 damage").

// optDecider reads OptionalDecider$ (gorge effDraw draw.go:80-144,
// effDealDamage damage_deal.go:91-118): the named player is asked yes / no
// before the effect, and a no skips the effect only (the SubAbility$ chain
// still runs). It lowers to an OpMayAsk over the one instruction the caller
// emits next; Obj names the decider (zero: the resolving controller). True
// and You are the controller; Opponent and TargetedController resolve
// through lc.defined; anything else refuses (gorge fails closed to a
// mandatory effect with a Note, which the kernel does not copy). guard is
// kop.MayGuardDraw or kop.MayGuardDamage.
func (lc *lowerCtx) optDecider(ps *params, guard int) error {
	v, ok := ps.get("OptionalDecider")
	if !ok {
		return nil
	}
	in := lInstr{Op: "MayAsk", C: guard, Skip: 1, Note: "you may"}
	switch spec := strings.TrimSpace(v); spec {
	case "True", "You":
	case "Opponent", "TargetedController", "TriggeredCardController":
		r, err := lc.defined(spec)
		if err != nil {
			return err
		}
		if r.Kind != rEachOpp && r.Kind != rTargetController && r.Kind != rTriggeredPlayer {
			return refuse("OptionalDecider %q as %s", v, r)
		}
		in.Obj = r
		in.Note = "decider may"
	default:
		return refuse("OptionalDecider %q", v)
	}
	lc.emit(in)
	return nil
}

// drawDecider gates a Draw on its OptionalDecider$: one drawing player only
// (gorge asks once for every drawing player; a multi-player set is out of
// scope). A declined draw remembers nothing, so RememberDrawn$ rides the same
// one-instruction skip as the draw itself.
func (lc *lowerCtx) drawDecider(ps *params, who lRef) error {
	if _, ok := ps.get("OptionalDecider"); !ok {
		return nil
	}
	switch who.Kind {
	case rYou, rTarget, rTargetController, rTriggeredPlayer:
	default:
		return refuse("OptionalDecider$ on a Draw by %s", who)
	}
	return lc.optDecider(ps, kop.MayGuardDraw)
}

// damageDecider gates a DealDamage on its OptionalDecider$: one recipient
// (a target slot, the controller or the source), the plain DealDamage op
// only, so the ask's Skip covers exactly the one instruction dealDamage
// emits.
func (lc *lowerCtx) damageDecider(ps *params, in *lInstr) error {
	if _, ok := ps.get("OptionalDecider"); !ok {
		return nil
	}
	for _, k := range []string{"DividedAsYouChoose", "DamageSource", "DamageMap", "ExcessSVar"} {
		if _, ok := ps.get(k); ok {
			return refuse("OptionalDecider$ beside %s$", k)
		}
	}
	if in.Op != "DealDamage" {
		return refuse("OptionalDecider$ on a %s", in.Op)
	}
	switch in.Obj.Kind {
	case rTarget, rYou, rSelf:
	default:
		return refuse("OptionalDecider$ on damage to %s", in.Obj)
	}
	return lc.optDecider(ps, kop.MayGuardDamage)
}
