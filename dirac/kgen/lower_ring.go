package kgen

// The Ring (CR 701.54): api RingTemptsYou (gorge effects/ring.go
// effRingTemptsYou) lowers as OpExt ExtRingTemptsYou over the tempted player;
// the Ring's reads are PlayerCountPropertyYou$RingTemptedYou (vRingTempted,
// value.go), the filter word IsRingbearer (cmpRingbearer, filter.go) and
// Mode$ RingTemptsYou (ringTemptsTrigger). The kernel's half is
// dirac/act_ring.go.

// ringTempts lowers RingTemptsYou: the Ring tempts the resolving controller.
// gorge reads no Defined$ (the corpus has none): the tempted player is always
// the controller, so a Defined$ or a target refuses rather than guess.
func (lc *lowerCtx) ringTempts(ps *params) error {
	if d, ok := ps.get("Defined"); ok {
		return refuse("RingTemptsYou Defined$ %q", d)
	}
	lc.emit(lInstr{Op: "RingTempts", Obj: lRef{Kind: rYou}, Note: "the Ring tempts the controller (gorge effRingTemptsYou)"})
	return nil
}

// ringTemptsTrigger lowers Mode$ RingTemptsYou (gorge trigmatch
// ringTemptsMatches): the Ring tempts a player (ValidPlayer$ You, or any when
// absent), and ValidCard$ gates on the creature chosen as the Ring-bearer,
// read with the trigger's source as "you" and "Other" (a trigger whose player
// chose no creature never fires on ValidCard$). The kernel scans the
// battlefield only, so a TriggerZones$ Graveyard line (Ringwraiths) refuses
// at lowerTriggered.
func (lc *lowerCtx) ringTemptsTrigger(tp *params, tr *lTrigger, readF func(string) (*lFilter, error)) error {
	if tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("RingTemptsYou trigger outside the battlefield")
	}
	w, ok := whoOfPlayer(tp.str("ValidPlayer"))
	if !ok {
		return refuse("RingTemptsYou ValidPlayer %q", tp.str("ValidPlayer"))
	}
	f, err := readF("ValidCard")
	if err != nil {
		return err
	}
	tr.Whose, tr.F = w, f
	return nil
}
