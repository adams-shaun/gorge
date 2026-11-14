package kgen

import "strings"

// noPrevention reads DealDamage NoPrevention$ True (CR 615.12, "the damage
// can't be prevented": Arrow Storm's raid half, Combust, Urza's Rage). gorge
// publishes the rider's bit around the Damage event it emits
// (effects/damage.go emitDamageEvent), and rules cantPreventDamage turns
// every prevention gate off for it. The kernel's node is OpDealDamageEx with
// kop.DmgNoPrevention, which sets the same "can't be prevented" fact for its
// damage (static_cantprevent.go noPrev). Only a plain damage node lowers: a
// divided, mapped or from-a-source node deals its damage elsewhere.
func (lc *lowerCtx) noPrevention(ps *params, in *lInstr) error {
	v, ok := ps.get("NoPrevention")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "True" {
		return refuse("NoPrevention$ %q", v)
	}
	if strings.HasPrefix(in.Note, "damage map") {
		return refuse("NoPrevention$ on a damage-map node")
	}
	if in.Op != "DealDamage" {
		return refuse("NoPrevention$ on a %s node", in.Op)
	}
	in.CS = strings.TrimPrefix(in.CS+"|NoPrevention", "|")
	return nil
}
