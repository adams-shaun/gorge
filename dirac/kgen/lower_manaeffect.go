package kgen

import (
	"strings"
)

// manaEffect lowers a DB$ Mana inside a resolving program (a trigger's or a
// spell's resolution: a triggered "add {B}{R}", or a spell's
// "Add {U}{U}{R}{R}{R}", CR 605.1a): one AddMana per produced colour into the
// controller's pool. PersistentMana$ True (mana kept as steps end until the
// turn ends) is B = 2, tallied in State.pm; PersistentUntilEndOfCombat$ is B = 1.
func (lc *lowerCtx) manaEffect(ps *params) error {
	switch {
	case lc.ab.Kind == akTriggered || lc.ab.Kind == akSpell:
	case lc.ab.Kind == akActivated && lc.dbManaActivatedOK():
		// a loyalty / targeted / library-moving ability is no mana ability
		// (CR 605.1a): gorge resolves it on the stack (lower_dbmanacontext.go)
	case lc.ab.Kind == akActivated:
		return refuse(refuseDbManaMA)
	case lc.ab.Kind == akMana:
		// CR 605.1a: an activated ability that could add mana is a mana
		// ability, and gorge's payment machinery treats its mana apart from
		// a resolving spell's.
		return refuse("DB$ Mana in an activated ability")
	default:
		return refuse("DB$ Mana outside a triggered ability")
	}
	if vt, ok := ps.get("ValidTgts"); ok && lc.ab.Kind == akSpell {
		// Rousing Refrain's "Add {R} for each card in target opponent's
		// hand": the spell declares its opponent slot first, so the amount's
		// TargetedPlayer$ names it. Only an opponents-only player slot lowers
		// (the amount reads the one opponent's hand); gorge asks the target
		// when the spell is cast.
		f, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if f.Players != plOpp || len(f.Clauses) != 0 || strings.TrimSpace(vt) != "Opponent" {
			return refuse("SP$ Mana ValidTgts$ %q", vt)
		}
		for _, k := range []string{"TargetMin", "TargetMax", "TgtZone", "TargetUnique"} {
			if _, ok := ps.get(k); ok {
				return refuse("SP$ Mana ValidTgts$ beside %s$", k)
			}
		}
		lc.addTarget(lTarget{F: f, Zone: zNone})
		ps.consume("ValidTgts")
	}
	rv, _ := ps.get("RestrictValid")
	if lc.out.Types&tArtifact != 0 && !lc.fbGranted && !dbManaActOnly(rv) { // a granted Firebending's source is the attacker, not this host (lower_firegrant.go)
		// An activated-only restriction (kop.SpmActivated) is the one typed
		// shape the kernel records: addManaRestr sets poolArtRestr.
		// an artifact source's mana is typed (gorge effects.ManaProducerTag
		// reads the printed face) and gorge's planned payment refuses a
		// pool holding typed mana (pay.PlanPoolOK); the kernel's pool
		// carries no typed bit, so every triggered DB$ Mana from an
		// artifact source diverges, fixed or Amount$ (Myr Moonvessel and
		// The Vision and Scarlet Witch on both lockstep surfaces;
		// Cathodion and Su-Chi)
		return refuse("DB$ Mana of an artifact source (typed mana)")
	}
	for _, k := range []string{"AddsKeywords", "AddsCounters"} {
		if _, ok := ps.get(k); ok {
			return refuse("DB$ Mana %s$", k)
		}
	}
	recipient := lRef{Kind: rYou}
	if v, ok := ps.get("Defined"); ok {
		// gorge ManaRecipients: no Defined$ means the activating player;
		// an explicit Defined$ You names the same player. Any other
		// selector (Opponent, ChosenPlayer, TriggeredCardController, ...)
		// names a different seat and stays refused.
		// TriggeredPlayer under a Phase trigger is the active player
		// (gorge definedSpecTriggeredPlayer): the controller for a
		// your-Phase trigger (Hulking Raptor), and the trigger's player
		// for an each-player one (Cheering Crowd).
		d := strings.TrimSpace(v)
		if d == "TriggeredPlayer" && lc.ab.Trigger != nil && lc.ab.Trigger.Event == "Phase" {
			switch {
			case lc.ab.Trigger.Whose == whoYou:
				// the controller
			case lc.ab.Trigger.Whose == whoAny && strings.HasPrefix(strings.TrimSpace(ps.str("UnlessCost")), "AddCounter<") &&
				ps.str("UnlessPayer") == "TriggeredPlayer" && ps.is("UnlessSwitched", "True"):
				// Cheering Crowd's "that player may ... if they do, they
				// add": the mana belongs to the trigger's player, read only for
				// the unless AddCounter body (any other each-player mana
				// recipient stays refused).
				recipient = lRef{Kind: rTriggeredPlayer}
			default:
				return refuse("DB$ Mana Defined$")
			}
		} else if d != "You" {
			return refuse("DB$ Mana Defined$")
		}
		ps.consume("Defined")
	}
	restrict, err := lc.dbManaRestrict(ps)
	if err != nil {
		return err
	}
	prod := ps.str("Produced")
	var persist lValue // B of the AddMana: 1 = firebending's red, kept until the end-of-combat step ends; 2 = PersistentMana$ True, kept until end of turn
	if v, ok := ps.get("PersistentMana"); ok {
		// gorge effMana: only the literal True is persistent (any other
		// value is a Note and ordinary mana); the units float in
		// State.pm beside the pool (mana_persistent.go)
		if v != "True" {
			return refuse("DB$ Mana PersistentMana$ %q", v)
		}
		if _, both := ps.get("PersistentUntilEndOfCombat"); both {
			return refuse("DB$ Mana PersistentMana$ beside PersistentUntilEndOfCombat$")
		}
		persist = cst(2)
	}
	if v, ok := ps.get("PersistentUntilEndOfCombat"); ok {
		// CR 702.189a: only gorge kwFirebending's expansion (an Attacks
		// Self trigger, Produced$ R, SubAbility$ __kwFirebendingMarker over
		// a lone DB$ ElementalBend | Verb$ fire) is read; any other use of
		// the param is refused
		if v != "True" {
			return refuse("PersistentUntilEndOfCombat$ %q", v)
		}
		if prod != "R" {
			return refuse("PersistentUntilEndOfCombat$ True beside Produced$ %q", prod)
		}
		sub := lc.curNode.Sub
		if lc.fbGranted && sub == nil && lc.curNode.Params["SubAbility"] == "" && lc.ab.Trigger != nil && lc.ab.Trigger.Event == "Attacks" {
			// a granted Firebending N (gorge trigger_granted.go): the bare
			// body, no marker sub-ability (lower_firegrant.go)
		} else if lc.curNode.Params["SubAbility"] != "__kwFirebendingMarker" || sub == nil || sub.API != "ElementalBend" ||
			sub.Params["Verb"] != "fire" || sub.Sub != nil || lc.ab.Trigger == nil || lc.ab.Trigger.Event != "Attacks" {
			return refuse("PersistentUntilEndOfCombat$ True outside a Firebending expansion")
		}
		lc.fbMarker = sub
		persist = cst(1)
	}
	if restrict != nil && persist.Kind != vNone {
		return refuse("DB$ Mana RestrictValid$ beside persistent mana")
	}
	if prod == "Any" || strings.HasPrefix(prod, "Combo ") {
		return lc.manaChoiceEffect(ps, prod, persist, restrict)
	}
	cols := strings.Fields(prod)
	if len(cols) == 0 {
		return refuse("mana %q", prod)
	}
	for _, c := range cols {
		if len(c) != 1 || !strings.Contains("WUBRGC", c) {
			return refuse("mana %q", prod)
		}
	}
	if _, ok := ps.get("Amount"); ok {
		// gorge effMana: Amount$ (Num, default 1) repeats the produced
		// symbols; one colour named once is Amount mana of it (the
		// triggered "add that much {R}"). Several symbols beside Amount$
		// stay refused.
		if len(cols) != 1 {
			return refuse("DB$ Mana Amount$ beside %q", prod)
		}
		n, err := lc.amount(ps, "Amount", 1)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "AddMana", Obj: recipient, A: n, B: persist, CS: cols[0], F: restrict})
		return nil
	}
	for _, c := range cols {
		lc.emit(lInstr{Op: "AddMana", Obj: recipient, A: cst(1), B: persist, CS: c, F: restrict})
	}
	return nil
}

// manaChoiceEffect lowers a resolving DB$ Mana over a colour choice (CR 106.3;
// a spell or triggered ability is no mana ability, CR 605.1a): Produced$ Combo <colours>
// asks one colour per mana, Produced$ Any one colour for the whole Amount$
// (gorge effects.askManaChoice). The loyalty path (lowerMana) emits the same
// ExtManaChoice.
func (lc *lowerCtx) manaChoiceEffect(ps *params, prod string, persist lValue, restrict *lFilter) error {
	colors, mode := "WUBRG", 1
	if prod != "Any" {
		colors, mode = strings.ReplaceAll(strings.TrimPrefix(prod, "Combo "), " ", ""), 0
		if colors == "Any" {
			colors = "WUBRG"
		}
	}
	if colors == "" || strings.Trim(colors, "WUBRG") != "" {
		return refuse("mana choice over %q", prod)
	}
	if restrict != nil && mode == 1 && !restrictActOnly(restrict) { // the activated-only marker lowers: Boommobile
		// Ashling, Rimebound (Produced$ Any, Amount$ 2): its restricted
		// units float beside the flip trigger's pay-{R} cost and Furnace
		// Whelp-style paid abilities, where gorge's offers differ from the
		// kernel's (the trigger-cost ask counts the restricted raw pool,
		// resPayNeedsMana; the ability offer vanishes): lockstep diverges
		// in about 1 game in 17, so the shape stays refused
		return refuse("DB$ Mana RestrictValid$ over Produced$ Any")
	}
	amt, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	// a constant amount is one to seven; a counted one is read when the effect
	// resolves (a zero adds nothing)
	if amt.isConst() && (amt.N < 1 || amt.N > 7) || len(colors) < 2 {
		return refuse("mana choice amount %q over %q", ps.str("Amount"), prod)
	}
	lc.emit(lInstr{Op: "ManaChoice", A: amt, B: persist, C: mode, CS: colors, F: restrict})
	return nil
}
