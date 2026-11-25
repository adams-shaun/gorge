package kgen

import "strings"

// excessDamage reads DealDamage ExcessDamage$ / ExcessDamageCondition$
// (CR 120.4a, 120.10; gorge effects emitObjectDamageExcess): "excess damage
// is dealt to that creature's controller instead". It lowers on a Bite (the
// source creature deals its power to one target creature, Ram Through) as
// kop.BiteExcessCtrl: the kernel splits the hit into the lethal amount to the
// creature (1 from a deathtouch source) and the rest to its controller, in
// one damage batch. ExcessDamageCondition$ is a filter on the damage source
// (Card.withTrample), the instruction's F. Refused: any recipient other than
// the damaged creature's controller (TargetedController /
// ThisTargetedController), and every other damage op (a spell's own damage
// has no source slot for the kernel's hit batch, so the two hits could not be
// one batch), a bite from a trigger object or a reflexive capture (their
// departed-source paths deal from last known information without the split)
// and a bite over several targets.
func (lc *lowerCtx) excessDamage(ps *params, in *lInstr, src lRef) error {
	ex, ok := ps.get("ExcessDamage")
	if !ok {
		return nil
	}
	switch strings.TrimSpace(ex) {
	case "TargetedController", "ThisTargetedController":
	default:
		return refuse("ExcessDamage$ %q", ex)
	}
	if in.Op != "Bite" {
		return refuse("ExcessDamage$ on a %s node", in.Op)
	}
	if src.Kind != rTarget {
		return refuse("ExcessDamage$ on a bite from %s", src)
	}
	if t := lc.ab.Targets[in.C]; !t.Max.isConst() || t.Max.N > 1 {
		return refuse("ExcessDamage$ on a bite over several targets")
	}
	if c, ok := ps.get("ExcessDamageCondition"); ok {
		f, err := lc.filter(strings.TrimSpace(c))
		if err != nil {
			return err
		}
		if f.Players != 0 {
			return refuse("ExcessDamageCondition$ %q names a player", c)
		}
		in.F = f
	}
	in.CS = strings.TrimPrefix(in.CS+"|ExcessCtrl", "|")
	return nil
}
