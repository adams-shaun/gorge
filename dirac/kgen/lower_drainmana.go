package kgen

// drainMana lowers api:DrainMana (gorge effDrainMana: each defined player
// loses all unspent mana) to ExtDrainMana. Power Sink names the targeted
// spell's controller; Mana Short the target player. Defined$ Player (every
// player) refuses: the kernel resolves one player (playerRef has no
// RefEachPlayer arm).
// Forge's optional DrainMana$ True (the drained mana is added to the
// activator) and RememberDrainedMana$ True change the effect and stay
// refused.
func (lc *lowerCtx) drainMana(ps *params) error {
	for _, k := range []string{"DrainMana", "RememberDrainedMana"} {
		if _, ok := ps.get(k); ok {
			return refuse("DrainMana %s$", k)
		}
	}
	d := "You"
	if v, ok := ps.get("Defined"); ok && v != "" {
		d = v
	}
	r, err := lc.defined(d)
	if err != nil {
		return err
	}
	switch r.Kind {
	case rYou, rEachOpp, rTarget, rTargetController, rTriggeredPlayer, rDefending:
	default:
		return refuse("DrainMana Defined$ %q is not a player selector", d)
	}
	lc.emit(lInstr{Op: "DrainMana", Obj: r})
	return nil
}
