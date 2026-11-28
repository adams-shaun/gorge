package kgen

// lower_repeatselectors.go: RepeatEach RepeatPlayers$ selectors that name a
// player set other than every player / opponent / the target slot's players.
// gorge repeatPlayers (engine/effects/choose_control.go) builds the selected
// set and then walks it in AliveFrom(controller) order, the controller first.

// repeatEachSelector maps a RepeatPlayers$ selector to the loop's player ref.
// andYou reports TargetedAndYou, whose loop the caller doubles
// (repeatAndYou).
//
//   - TargetedAndYou: gorge adds the targets' players and the controller.
//     Only a slot that is exactly one opponent player target is lowered
//     (Keen Duelist's ValidTgts$ Opponent): the set is then the controller
//     and that opponent, two distinct seats, the controller first.
//   - OppNonRememberedController: the controller's opponents, AliveFrom
//     order, less every remembered object's controller (Skull Storm's
//     sacrificed creatures). The kernel's RefOppNonRemCtrl is that whole set
//     (dirac filter_defined.go oppsNonRemCtrl), at any seat count.
//   - TargetedController: the controllers of every target, deduplicated, in
//     AliveFrom(controller) order (gorge repeatPlayersTargeted then the
//     AliveFrom walk). The kernel's target-slot player set reads only player
//     targets, so RefTgtControllers adds every slot's object targets (and the
//     extra picks of a multi-object slot); Disorienting Choice's target slot
//     is the carrier.
func (lc *lowerCtx) repeatEachSelector(rp string) (who lRef, andYou bool, err error) {
	switch rp {
	case "TargetedAndYou":
		if who, err = lc.repeatEachTargetedSlot(); err != nil {
			return lRef{}, false, err
		}
		t := &lc.ab.Targets[who.N]
		if one := t.Max.Kind == vNone || t.Max.isConst() && t.Max.N == 1; t.F.Players != plOpp || !one {
			return lRef{}, false, refuse("RepeatEach TargetedAndYou: the target slot is not one opponent")
		}
		return who, true, nil
	case "OppNonRememberedController":
		return lRef{Kind: rOppNonRemCtrl}, false, nil
	case "TargetedController":
		if len(lc.ab.Targets) == 0 {
			return lRef{}, false, refuse("RepeatEach TargetedController: no target slot")
		}
		for i := range lc.ab.Targets {
			t := &lc.ab.Targets[i]
			// The kernel's tgtControllers reads each target's live controller
			// off the object record; a spell or ability target is a stack code
			// with no object slot (and gorge PlayerOf would name its
			// controller from last-known info this loop does not carry), so a
			// slot that can name one refuses by name (a permanent, card or
			// player slot is fine).
			if t.Spell || t.StackAb || t.AbOnly || t.PermOrSpell || t.Zone == zStack {
				return lRef{}, false, refuse("RepeatEach TargetedController: target slot %d can name a spell or ability", i)
			}
		}
		return lRef{Kind: rTgtControllers}, false, nil
	}
	return lRef{}, false, refuse("RepeatEach RepeatPlayers %q", rp)
}

// repeatAndYou turns the RepeatEach loop at Prog[at:] (its target slot ref
// already in Obj) into gorge's TargetedAndYou order: the loop runs first
// over the controller, then a copy of it over the targeted opponent. Every
// Skip is relative to its own instruction, so the copy needs no patching.
func (lc *lowerCtx) repeatAndYou(at int) {
	loop := append([]lInstr(nil), lc.ab.Prog[at:]...)
	lc.ab.Prog[at].Obj = lRef{Kind: rYou}
	lc.ab.Prog = append(lc.ab.Prog, loop...)
}

// peekAndRevealAndYou lowers PeekAndReveal Defined$ TargetedAndYou (Keen
// Duelist): gorge's alias TargetedPlayer & You reveals the targeted
// opponent's top cards, then the controller's, each with gorge effReveal's
// public reveal (no ask; RememberRevealed$ appends to the remembered set).
// Only that public arm is lowered: a look, an optional reveal or a
// remembered peek over those two players refuses (seat-exact: the ref and
// You name the same two seats at any seat count).
func (lc *lowerCtx) peekAndRevealAndYou(ps *params, who lRef) error {
	for _, k := range []string{"NoReveal", "RevealOptional", "RevealValid", "RememberPeeked"} {
		if _, ok := ps.get(k); ok {
			return refuse("PeekAndReveal TargetedAndYou with %s$", k)
		}
	}
	n, err := lc.amount(ps, "PeekAmount", 1)
	if err != nil {
		return err
	}
	ps.consume("NoPeek") // unread in gorge
	cs := "Public"
	if ps.is("RememberRevealed", "True") {
		cs += "|Remember"
	} else if _, ok := ps.get("RememberRevealed"); ok {
		return refuse("PeekAndReveal RememberRevealed %q", ps.str("RememberRevealed"))
	}
	lc.emit(lInstr{Op: "Peek", Obj: who, A: n, CS: cs})
	lc.emit(lInstr{Op: "Peek", Obj: lRef{Kind: rYou}, A: n, CS: cs})
	return nil
}
