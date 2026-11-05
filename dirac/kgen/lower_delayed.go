package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// delayedUpkeep lowers a DelayedTrigger Mode$ Phase | Phase$ Upkeep (gorge
// effDelayedTrigger: a one-shot registration fired at the next StepChange
// into an upkeep whose active player matches ValidPlayer$; NextTurn$ True
// only bounds it to a later turn, which a registration made after this
// turn's upkeep began already is). ValidPlayer$ Player (or absent) is the
// next upkeep of any player, You the controller's next upkeep. Only the
// form without RememberObjects$ lowers, and only when the delayed body
// reads no remembered object (gorge hands it the resolving chain's
// capture, which the kernel's program does not carry).
func (lc *lowerCtx) delayedUpkeep(ps *params) error {
	if lc.ab.Kind == akMana {
		// the kernel adds a mana ability's mana and runs no program
		return refuse("delayed upkeep trigger in a mana ability")
	}
	when := "NextUpkeep"
	switch vp := ps.str("ValidPlayer"); vp {
	case "", "Player":
	case "You":
		when = "YourNextUpkeep"
	default:
		return refuse("delayed upkeep trigger ValidPlayer %q", vp)
	}
	if ro, ok := ps.get("RememberObjects"); ok {
		return refuse("delayed upkeep trigger RememberObjects %q", ro)
	}
	ps.consume("NextTurn")
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("delayed trigger Execute %q", ps.str("Execute"))
	}
	if lc.chainReadsRemembered(exec) {
		return refuse("delayed upkeep trigger body reads Remembered")
	}
	lat := lc.newLatent("DelayedTrigger")
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "DelayedTrigger", C: lat.Index, CS: when})
	return nil
}

// delayedEndCombat lowers a DelayedTrigger Mode$ Phase | Phase$ EndCombat
// (gorge effDelayedTrigger: a one-shot registration fired at the next
// StepChange into an end of combat step, EarliestAfter the current step;
// ValidPlayer$ Player, or absent, matches any active player; ThisTurn$ is
// read only beside DelayedTriggerDefinedPlayer$, so alone it changes
// nothing). RememberObjects$ absent keeps the resolving chain's capture,
// which the kernel's program does not carry: the body must then read no
// remembered object. A RememberObjects$ naming one object the lowering
// resolves (the trigger's card or attacker, one target) is the delayed
// program's remembered object (RefDelayedRemembered in the body).
func (lc *lowerCtx) delayedEndCombat(ps *params) error {
	if lc.ab.Kind == akMana {
		return refuse("delayed end-of-combat trigger in a mana ability")
	}
	switch vp := ps.str("ValidPlayer"); vp {
	case "", "Player":
	default:
		return refuse("delayed end-of-combat trigger ValidPlayer %q", vp)
	}
	ps.consume("ThisTurn")
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("delayed trigger Execute %q", ps.str("Execute"))
	}
	if ps.str("OncePerSource") != "" || ps.str("RememberChain") != "" {
		// Decayed's registration (lower_decayed.go): the only shape that
		// carries these, and the kernel reads neither
		if !decayedRegistration(ps) {
			return refuse("delayed end-of-combat trigger OncePerSource$/RememberChain$ outside Decayed")
		}
		ps.consume("OncePerSource", "RememberChain")
	}
	var obj lRef
	if ro, ok := ps.get("RememberObjects"); ok {
		switch ro {
		case "TriggeredAttackerLKICopy", "TriggeredAttacker", "TriggeredCard", "TriggeredCardLKICopy", "Targeted":
		default:
			return refuse("delayed trigger RememberObjects %q", ro)
		}
		r, err := lc.defined(ro)
		if err != nil {
			return err
		}
		if r.Kind != rTriggered && r.Kind != rTarget {
			return refuse("delayed trigger RememberObjects %q", ro)
		}
		if r.Kind == rTarget && lc.ab.Targets[r.N].F != nil && len(lc.ab.Targets[r.N].F.Clauses) == 0 {
			return refuse("delayed trigger RememberObjects %q: a player target", ro)
		}
		obj = r
	} else if lc.chainReadsRemembered(exec) {
		return refuse("delayed end-of-combat trigger body reads Remembered")
	}
	lat := lc.newLatent("DelayedTrigger")
	lat.DelayedObjOnly = delayedObjOnly(ps.str("RememberObjects"))
	lat.DelayedSelf = delayedSelf(lc.ab.Trigger, ps.str("RememberObjects"))
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	in := lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: "NextEndCombat"}
	if decayedRegistration(ps) {
		in.B = cst(2) // incarnation-tracked (lower_decayed.go)
	}
	lc.emit(in)
	return nil
}

// delayedBeginCombat lowers a DelayedTrigger Mode$ Phase | Phase$ BeginCombat |
// ThisTurn$ True | RememberObjects$ Targeted (The Wrath of Khan's "at the
// beginning of the next combat this turn, gain control of that creature":
// gorge effDelayedTrigger registers a one-shot at the next StepChange into a
// beginning of combat step, bounded to this turn). The delayed program's
// remembered object is the one battlefield target; the registration is
// dropped at the turn change (kop.DelayBeginCombat). Only that shape lowers.
func (lc *lowerCtx) delayedBeginCombat(ps *params) error {
	if lc.ab.Kind == akMana {
		return refuse("delayed beginning-of-combat trigger in a mana ability")
	}
	if !ps.is("ThisTurn", "True") {
		return refuse("delayed beginning-of-combat trigger without ThisTurn$")
	}
	if vp := ps.str("ValidPlayer"); vp != "" {
		return refuse("delayed beginning-of-combat trigger ValidPlayer %q", vp)
	}
	if ro := ps.str("RememberObjects"); ro != "Targeted" {
		return refuse("delayed beginning-of-combat trigger RememberObjects %q", ro)
	}
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("delayed trigger Execute %q", ps.str("Execute"))
	}
	obj, err := lc.delayedRemember("Targeted", exec)
	if err != nil {
		return err
	}
	lat := lc.newLatent("DelayedTrigger")
	lat.DelayedObjOnly = delayedObjOnly("Targeted")
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: "NextBeginCombat"})
	return nil
}

// delayedRemember lowers an end-step DelayedTrigger's other
// RememberObjects$ forms (gorge effDelayedTrigger: absent keeps the
// resolving chain's capture; another value replaces it with what
// knownDefinedTargets names): absent with a body that reads no remembered
// object (the program needs none); Remembered in a spell or an activated
// ability (no trigger capture: the chain's remembered set, as
// RememberedLKI); the trigger's own object (TriggeredCard /
// TriggeredCardLKICopy, RefTriggerObj: the pending trigger carries it as
// RefDelayedRemembered with its LKI snapshot); one target object
// (Targeted / ParentTarget). Other forms refuse.
func (lc *lowerCtx) delayedRemember(ro string, exec *cards.SA) (lRef, error) {
	switch ro {
	case "":
		if lc.chainReadsRemembered(exec) {
			return lRef{}, refuse("delayed trigger without RememberObjects$: the body reads Remembered")
		}
		return lRef{}, nil
	case "Remembered":
		if lc.ab.Kind != akSpell && lc.ab.Kind != akActivated &&
			!(delayedGroupSac(exec) && lc.hasRememberWriter()) {
			// a triggered ability's plain Remembered group is its fire-time
			// capture unless this resolution wrote its own set and the delayed
			// body is the group sacrifice over it (DigUntil RememberFound$ True
			// then SacrificeAll Defined$ DelayTriggerRememberedLKI, Will Riker)
			return lRef{}, refuse("delayed trigger RememberObjects %q in a triggered ability (the trigger capture)", ro)
		}
		if lc.czForgetSet {
			// a ChangeZoneAll ForgetOtherRemembered$ leaves a many-object
			// set (Random Encounter's milled creatures): the kernel's
			// DpMany program carries up to four and its body is the
			// many-object return, so the body must be exactly that
			// (lower_changezoneall.go; dirac/trig_delayedmany.go)
			if exec == nil || exec.API != "ChangeZone" || exec.Sub != nil || len(exec.Params) != 3 ||
				exec.Params["Defined"] != "DelayTriggerRememberedLKI" ||
				exec.Params["Origin"] != "Battlefield" || exec.Params["Destination"] != "Hand" {
				return lRef{}, refuse("delayed trigger over a ChangeZoneAll ForgetOtherRemembered$ set: the body is not one ChangeZone Battlefield->Hand of DelayTriggerRememberedLKI")
			}
		}
		return lRef{Kind: rRemembered}, nil
	case "TriggeredCard", "TriggeredCardLKICopy":
		// gorge effDelayedTrigger's RememberObjects$ replaces the chain's
		// capture with the trigger's card (knownDefinedTargets). The delayed
		// program's Obj is the trigger object (RefTriggerObj); the pending
		// trigger carries it as RefDelayedRemembered and its LKI snapshot.
		r, err := lc.defined(ro)
		if err != nil {
			return lRef{}, err
		}
		if r.Kind != rTriggered {
			return lRef{}, refuse("delayed trigger RememberObjects %q", ro)
		}
		return r, nil
	case "Targeted", "ParentTarget":
		r, err := lc.defined(ro)
		if err != nil {
			return lRef{}, err
		}
		t := lc.ab.Targets[r.N]
		if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
			return lRef{}, refuse("delayed trigger RememberObjects %q: target slot %d", ro, r.N)
		}
		return r, nil
	}
	return lRef{}, refuse("delayed trigger RememberObjects %q", ro)
}

// kernelOpLeaveReturn is the kernel's internal leave-return act
// (dirac/trig_leavereturn.go opLeaveReturn), carried in an
// OpDelayedTrigger's C.
const kernelOpLeaveReturn = 230

// delayedLeaveReturn lowers Melira's "When it's put into a graveyard this
// turn, return that card to the battlefield under its owner's control"
// (gorge's ChangesZone delayed trigger over the remembered target): only the
// exact shape, a one-object battlefield target slot remembered, Origin$
// Battlefield, Destination$ Graveyard, ThisTurn$ True, and a plain
// graveyard-to-battlefield return of the triggered card.
func (lc *lowerCtx) delayedLeaveReturn(ps *params) error {
	if ps.str("ValidCard") != "Card.IsTriggerRemembered" || ps.str("Origin") != "Battlefield" ||
		ps.str("Destination") != "Graveyard" || !ps.is("ThisTurn", "True") {
		return refuse("delayed trigger ChangesZone shape")
	}
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec != nil && exec.API != "ChangeZone" {
		return lc.delayedDies(ps, exec)
	}
	if exec == nil || exec.Sub != nil {
		return refuse("delayed trigger ChangesZone Execute %q", ps.str("Execute"))
	}
	bp := newParams(exec.Params)
	if bp.str("Origin") != "Graveyard" || bp.str("Destination") != "Battlefield" || bp.str("Defined") != "TriggeredCard" {
		return refuse("delayed trigger ChangesZone return shape")
	}
	bp.consume("Origin", "Destination", "Defined", "SpellDescription", "StackDescription")
	if left := bp.unread(); len(left) > 0 {
		return refuse("delayed trigger ChangesZone return params %v", left)
	}
	if ps.str("RememberObjects") != "Targeted" {
		return refuse("delayed trigger ChangesZone RememberObjects %q", ps.str("RememberObjects"))
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("delayed trigger ChangesZone without its own target")
	}
	if t := lc.ab.Targets[obj.N]; !t.Max.isConst() || t.Max.N != 1 {
		return refuse("delayed trigger ChangesZone target count")
	}
	ps.consume("StackDescription", "TgtPrompt")
	lc.emit(lInstr{Op: "DelayLeaveReturn", Obj: obj})
	return nil
}

// delayedDies lowers "when that creature dies this turn, <effect>" (gorge's
// ChangesZone delayed trigger over the remembered target, any body but
// Melira's return): the registration remembers one battlefield target, its
// own (Fatal Fissure) or, for a sub-ability with none, the chain's single
// one-object slot (Grim Javelineer, Turn Inside Out, Scarblade's Malice,
// Felonious Rage); the body is a latent program fired when that object goes
// from the battlefield to a graveyard this turn (kop.DelayDiesThisTurn).
func (lc *lowerCtx) delayedDies(ps *params, exec *cards.SA) error {
	if ps.str("RememberObjects") != "Targeted" {
		return refuse("delayed trigger ChangesZone RememberObjects %q", ps.str("RememberObjects"))
	}
	var obj lRef
	if _, own := ps.get("ValidTgts"); own {
		var err error
		if obj, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
		if obj.Kind != rTarget {
			return refuse("delayed dies trigger target shape")
		}
		ps.consume("TgtPrompt")
	} else {
		if _, ok := ps.get("Defined"); ok {
			return refuse("delayed dies trigger with Defined$")
		}
		if len(lc.ab.Targets) != 1 {
			return refuse("delayed dies trigger over %d chain target slots", len(lc.ab.Targets))
		}
		obj = lRef{Kind: rTarget, N: 0}
	}
	if t := lc.ab.Targets[obj.N]; !t.Max.isConst() || t.Max.N != 1 || t.Zone != zBattlefield {
		return refuse("delayed dies trigger target count")
	}
	lat := lc.newLatent("DelayedTrigger")
	if err := lc.withAbility(lat, func() error { return lc.chain(exec, nil) }); err != nil {
		return err
	}
	ps.consume("StackDescription")
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: "DiesThisTurn"})
	return nil
}
