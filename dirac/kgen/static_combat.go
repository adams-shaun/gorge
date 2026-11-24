package kgen

import (
	"strings"
)

// The bodies of the combat statics (static_mode.go).
func init() {
	staticMode("CantBlock", (*staticCtx).modeCantBlock)
	staticMode("MinMaxBlocker", (*staticCtx).modeMinMaxBlocker)
	staticMode("CantBlockBy", (*staticCtx).modeCantBlockBy)
	staticMode("CantAttack", (*staticCtx).modeCantAttack)
	staticMode("CantAttackUnless", (*staticCtx).modeCantAttackUnless)
	staticMode("CantBlockUnless", (*staticCtx).modeCantBlockUnless)
	staticMode("CanAttackIfHaste", (*staticCtx).modeCanAttackIfHaste)
	staticMode("CanAttackDefender", (*staticCtx).modeCanAttackDefender)
	staticMode("MustAttack", (*staticCtx).modeMustAttack)
	staticMode("UntapOtherPlayer", (*staticCtx).modeUntapOtherPlayer)
	staticMode("CombatDamageToughness", (*staticCtx).modeCombatDamageToughness)
	staticMode("AssignNoCombatDamage", (*staticCtx).modeAssignNoCombatDamage)
	staticMode("AssignCombatDamageAsUnblocked", (*staticCtx).modeAssignCombatDamageAsUnblocked)
	staticMode("ActivateAbilityAsIfHaste", (*staticCtx).modeActivateAbilityAsIfHaste)
	staticMode("CantBeSuspected", (*staticCtx).modeActivateAbilityAsIfHaste)
}

func (sc *staticCtx) modeCantBlock() ([]lStatic, bool, error) {
	readF := sc.readF
	var err error
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	sc.sp.readInertValidSA()
	return nil, false, nil
}

func (sc *staticCtx) modeMinMaxBlocker() ([]lStatic, bool, error) {
	sp := sc.sp
	readF := sc.readF
	var err error
	// Max$ 1 only (gorge MinMaxBlockerBounds reads the
	// literal smallest Max$; the corpus's only value is 1)
	if sp.str("Min") != "" {
		return nil, true, refuse("MinMaxBlocker with both Min and Max")
	}
	if v := sp.str("Max"); v != "1" {
		return nil, true, refuse("MinMaxBlocker Max %q", v)
	}
	sp.consume("Max", "Secondary")
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantBlockBy() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	readF := sc.readF
	var err error
	if sc.st.F, err = readF("ValidAttacker"); err != nil {
		return nil, true, err
	}
	if vb, ok := sp.get("ValidBlocker"); ok {
		rest, mode := pwVsSrcSplit(vb, lc.face.SVars) // lower_powercompare.go
		if sc.st.F2, err = lc.filter(rest); err != nil {
			return nil, true, err
		}
		sc.st.PwVsSrc = mode
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantAttack() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.cantAttack(sp, &sc.st); err != nil {
		return nil, true, err
	}
	sp.readInertValidSA()
	return nil, false, nil
}

func (sc *staticCtx) modeCantAttackUnless() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.cantAttackUnless(sp, &sc.st); err != nil {
		return nil, true, err // lower_attacktax.go
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantBlockUnless() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.cantBlockUnless(sp, &sc.st); err != nil {
		return nil, true, err // lower_blocktax.go
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCanAttackIfHaste() ([]lStatic, bool, error) {
	s := sc.s
	sp := sc.sp
	readF := sc.readF
	var err error
	// gorge canAttackIfHaste (rules/combat/attack.go): ValidCard$ (absent:
	// every creature) scopes the exempt creatures; a present ValidTarget$
	// is tried as a player spec on the defender, then as the attacked
	// permanent. Only the Frenzied Saddlebrute spelling lowers: it names
	// the defender player (an opponent of the static's controller) and the
	// planeswalkers that player controls, so the exemption depends on the
	// defender seat alone. gorge skips the line (inert) when it carries
	// a param outside its whitelist, so any other param refuses here
	// rather than lower a condition gorge never reads.
	sp.consume("Description", "Secondary")
	for k := range s.Params {
		switch k {
		case "Mode", "ValidCard", "ValidTarget", "Description", "Secondary":
		default:
			return nil, true, refuse("CanAttackIfHaste %s$ (gorge reads the line only over ValidCard$ and ValidTarget$)", k)
		}
	}
	if vc, ok := sp.get("ValidCard"); ok {
		if sc.st.F, err = readF("ValidCard"); err != nil {
			return nil, true, err
		}
		if sc.st.F == nil || len(sc.st.F.Clauses) == 0 || sc.st.F.Players != 0 {
			return nil, true, refuse("CanAttackIfHaste ValidCard %q", vc)
		}
	}
	if tv, ok := sp.get("ValidTarget"); ok {
		if strings.TrimSpace(tv) != "Opponent,Planeswalker.OppCtrl" {
			return nil, true, refuse("CanAttackIfHaste ValidTarget %q", tv)
		}
		sc.st.Flags = append(sc.st.Flags, "OppDef")
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCanAttackDefender() ([]lStatic, bool, error) {
	s := sc.s
	sp := sc.sp
	readF := sc.readF
	var err error
	// gorge combat.AttackAllowedThroughDefender: ValidCard$ (or
	// ValidCards$) must match the creature, ValidAttacked$ must admit
	// the defender. ValidAttacked$ stays unread (the unread-param check
	// refuses it); a player-only ValidCard$ filter refuses.
	if _, ok := sp.get("ValidCard"); !ok {
		return nil, true, refuse("CanAttackDefender without ValidCard$")
	}
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if sc.st.F == nil || len(sc.st.F.Clauses) == 0 || sc.st.F.Players != 0 {
		return nil, true, refuse("CanAttackDefender ValidCard %q", s.Params["ValidCard"])
	}
	return nil, false, nil
}

func (sc *staticCtx) modeMustAttack() ([]lStatic, bool, error) {
	readF := sc.readF
	var err error
	if sc.st.F, err = readF("ValidCreature"); err != nil {
		return nil, true, err
	}
	return nil, false, nil
}

func (sc *staticCtx) modeUntapOtherPlayer() ([]lStatic, bool, error) {
	s := sc.s
	readF := sc.readF
	var err error
	// ValidCard$ over the non-active player's tapped permanents, matched
	// from the static's source and controller (gorge
	// untapOtherStaticsMatch); absent means Card.Self. A player-only
	// filter reads nothing the permanent scan can evaluate.
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if sc.st.F == nil {
		sc.st.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
	} else if sc.st.F.Players != 0 {
		return nil, true, refuse("UntapOtherPlayer ValidCard %q", s.Params["ValidCard"])
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCombatDamageToughness() ([]lStatic, bool, error) {
	readF := sc.readF
	var err error
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if sc.st.F == nil {
		return nil, true, refuse("CombatDamageToughness without ValidCard$")
	}
	return nil, false, nil
}

// modeAssignNoCombatDamage lowers CR 510.1a's "assigns no combat damage"
// (Ophidian, the Laccolith family): the ValidCard$ filter names the creature
// whose assignment the kernel's combat damage step zeroes (gorge
// effectCombatStatic's AssignNoCombatDamage restriction, read by
// rules' combatDamageAmount).
func (sc *staticCtx) modeAssignNoCombatDamage() ([]lStatic, bool, error) {
	readF := sc.readF
	var err error
	sp := sc.sp
	sp.consume("Description")
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if sc.st.F == nil {
		return nil, true, refuse("AssignNoCombatDamage without ValidCard$")
	}
	return nil, false, nil
}

// modeAssignCombatDamageAsUnblocked lowers CR 510.1c's optional "you may have
// CARDNAME assign its combat damage as though it weren't blocked" (Thorn
// Elemental, Rhox): the ValidCard$ filter names the creatures the kernel's
// combat damage step offers the election over (gorge asUnblockedStaticMatches
// -> asUnblockedNeeding -> askNextCombatAsk, combat_asgngrant.go's
// nextElection), and the shared readCond has built any IsPresent$ gate into
// the static's condition (Siege Behemoth's Card.Self+attacking). Every
// printed corpus carrier spells Optional$ True — the controller's election.
// A static without it is gorge's mandatory auto-accept reading
// (asUnblockedStaticMatches' no-Optional arm), which the kernel has no ask
// for on a printed static: refuse rather than ask where gorge auto-accepts.
// The Effect-delivered registrations (Outmaneuver, Gurzigost) lower through
// lower_asgngrant.go's ExtAsgnGrant instead, which refuses the Optional$ True
// election shape: gorge keeps the election for it, the kernel has no
// Effect-side ask.
func (sc *staticCtx) modeAssignCombatDamageAsUnblocked() ([]lStatic, bool, error) {
	sp := sc.sp
	readF := sc.readF
	var err error
	if !sp.is("Optional", "True") {
		return nil, true, refuse("AssignCombatDamageAsUnblocked Optional %q", sp.str("Optional"))
	}
	sp.consume("Optional")
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if sc.st.F == nil {
		return nil, true, refuse("AssignCombatDamageAsUnblocked without ValidCard$")
	}
	return nil, false, nil
}

func (sc *staticCtx) modeActivateAbilityAsIfHaste() ([]lStatic, bool, error) {
	s := sc.s
	sp := sc.sp
	readF := sc.readF
	var err error
	// gorge activatesAsIfHaste (rules/activateasifhaste.go) skips an
	// unscoped static (fail closed); cantBeSuspected
	// (rules/cantbesuspected.go) reads an unscoped one as blanket. The
	// corpus always scopes both: refuse the bare form rather than pick.
	sp.consume("Secondary")
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if sc.st.F == nil || len(sc.st.F.Clauses) == 0 || sc.st.F.Players != 0 {
		return nil, true, refuse("%s ValidCard %q", s.Mode, s.Params["ValidCard"])
	}
	return nil, false, nil
}
