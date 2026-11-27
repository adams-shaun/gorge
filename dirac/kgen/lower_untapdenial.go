package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// untapDenialAffected is the one Affected$ list this lowering reads: the
// creatures and lands of the remembered player (Exhaustion). The other
// spellings gorge registers an UntapDenial for (Misstep's creatures, Mana
// Vapors' lands, Icebreaker Kraken's artifacts and creatures) refuse by name.
var untapDenialAffected = map[string]bool{
	"Creature.RememberedPlayerCtrl": true,
	"Land.RememberedPlayerCtrl":     true,
}

// effectUntapDenial lowers a DB$ Effect that keeps the targeted player's
// permanents tapped through their next untap step, beside a Triggers$ line
// that is the Effect's own end: Mode$ Phase | Phase$ Untap | ValidPlayer$
// Player.IsRemembered, exiling the Effect from the command zone. Two bodies
// spell the restriction:
//
//   - gorge's UntapDenial grant (engine/effects effEffect: Affected$ over
//     RememberedPlayerCtrl with AddHiddenKeyword$ "This card doesn't untap
//     during your next untap step.", registered as a restriction rules'
//     untap-step scan reads): the creatures and lands of the remembered
//     player (Exhaustion), OpSkipUntap C=SkipUntapPlayerCL;
//   - a ReplacementEffects$ line Event$ Untap | Layer$ CantHappen |
//     ValidCard$ Creature | ValidStepTurnToController$ Player.IsRemembered,
//     which replaces the untap of the remembered player's creatures
//     (Blinding Beam), OpSkipUntap C=SkipUntapPlayerCreatures.
//
// Both end with the untap step they skip (rules/turn.go expireUntapStepEffects),
// which is why the self-exile trigger emits nothing. It reports whether the
// Effect had that shape; any other static- or replacement-beside-trigger
// Effect returns false for the pair lowering to read, and a half-matching one
// refuses by name.
func (lc *lowerCtx) effectUntapDenial(ps *params, tr string) (bool, error) {
	list, hasStatic := ps.get("StaticAbilities")
	replName, hasRepl := ps.get("ReplacementEffects")
	no := func() (bool, error) {
		// an unsuccessful read must not mark the params consumed: the pair
		// lowering (or the node check) still names them
		delete(ps.used, "StaticAbilities")
		delete(ps.used, "ReplacementEffects")
		return false, nil
	}
	if !hasStatic && !hasRepl {
		return no()
	}
	why := func(more string) (bool, error) {
		if hasStatic {
			return true, refuse("Effect untap denial %q (%s)", list, more)
		}
		return true, refuse("Effect untap denial %q (%s)", replName, more)
	}
	if hasStatic && hasRepl {
		// one Effect cannot be both a grant and a replacement here: the
		// static body is the grant shape and the replacement body is the
		// untap-step skip, and each branch would leave the other unread
		return why("both StaticAbilities$ and ReplacementEffects$")
	}
	if lc.ab.Kind == akMana {
		return why("in a mana ability")
	}
	captures := kop.SkipUntapPlayerCL
	if hasStatic {
		ss, err := lc.effectStaticBodies(list)
		if err != nil {
			return no() // the pair lowering names it
		}
		if len(ss) != 1 || ss[0].Mode != "Continuous" ||
			!cards.IsHiddenUntapNextStepKeyword(strings.TrimSpace(ss[0].Params["AddHiddenKeyword"])) {
			return no()
		}
		sp := newParams(ss[0].Params)
		sp.consume("Mode", "Description", "AffectedZone", "AddHiddenKeyword")
		if z, ok := ss[0].Params["AffectedZone"]; ok && strings.TrimSpace(z) != "Battlefield" {
			return why("AffectedZone$ " + z)
		}
		seen := map[string]bool{}
		for _, a := range strings.Split(sp.str("Affected"), ",") {
			a = strings.TrimSpace(a)
			if !untapDenialAffected[a] || seen[a] {
				return why("Affected$ " + ss[0].Params["Affected"])
			}
			seen[a] = true
		}
		if len(seen) != len(untapDenialAffected) {
			return why("Affected$ " + ss[0].Params["Affected"])
		}
		if left := sp.unread(); len(left) > 0 {
			return why("static unread " + strings.Join(left, ","))
		}
	} else {
		r, ok := cards.ParseReplacementLine(lc.face.SVars[strings.TrimSpace(replName)])
		if !ok || r.Event != "Untap" {
			return no()
		}
		rp := newParams(r.Params)
		rp.consume("Event")
		if !rp.is("Layer", "CantHappen") || strings.TrimSpace(rp.str("ValidCard")) != "Creature" ||
			rp.str("ValidStepTurnToController") != "Player.IsRemembered" {
			return no() // another Untap replacement keeps its own route
		}
		if _, ok := rp.get("ReplaceWith"); ok {
			return no()
		}
		rp.consume("ValidCard", "ValidStepTurnToController", "Description")
		if left := rp.unread(); len(left) > 0 {
			return why("replacement unread " + strings.Join(left, ","))
		}
		ps.consume("ReplacementEffects")
		captures = kop.SkipUntapPlayerCreatures
	}
	if !ps.is("Duration", "Permanent") || ps.str("RememberObjects") != "Targeted" {
		return why("Duration$ " + ps.m["Duration"] + " RememberObjects$ " + ps.m["RememberObjects"])
	}
	if err := lc.untapDenialEnd(tr); err != nil {
		return true, err
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	if obj.Kind != rTarget {
		return why("RememberObjects$ Targeted over " + obj.String())
	}
	if t := lc.ab.Targets[obj.N]; t.F == nil || len(t.F.Clauses) != 0 || t.F.Players == 0 || !t.Max.isConst() || t.Max.N != 1 {
		return why("the target is not one player")
	}
	ps.consume("Triggers", "RememberObjects", "Duration", "Name", "Image", "Stackable")
	note := "the target player's creatures and lands"
	if !hasStatic {
		note = "the target player's creatures"
	}
	// The Effect's Phase$ Untap self-exile is Static$ True: gorge resolves it
	// inline as CR 611.2a duration bookkeeping and never puts it on the stack,
	// so the kernel emits nothing for it (the Effect is no object here, and
	// the restriction ends with the untap step it skips).
	lc.emit(lInstr{Op: "SkipUntap", Obj: obj, C: captures, Note: note})
	return true, nil
}

// untapDenialEnd checks the Effect's Triggers$ line is exactly the end the
// restriction's lowering models: the remembered player's untap step exiles
// the Effect (Exhaustion's RemoveEffect).
func (lc *lowerCtx) untapDenialEnd(tr string) error {
	t, ok := cards.ParseTriggerLine(lc.face.SVars[strings.TrimSpace(tr)])
	if !ok || t.Mode != "Phase" {
		return refuse("Effect untap denial trigger %q", tr)
	}
	if t.Effect == nil {
		t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"])
	}
	if t.Effect == nil {
		return refuse("Effect untap denial trigger %q has no body", tr)
	}
	// TriggerZones$ Command is the printed spelling; an Effect's registered
	// trigger is in the command zone anyway, so an absent one is the same
	// shape (Blinding Beam's RestoreSight).
	if tz, ok := t.Params["TriggerZones"]; ok && strings.TrimSpace(tz) != "Command" {
		return refuse("Effect untap denial trigger %q trigger zones %q", tr, tz)
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Phase", "ValidPlayer", "TriggerZones", "Static", "Execute", "TriggerDescription")
	if !tp.is("Phase", "Untap") || !tp.is("ValidPlayer", "Player.IsRemembered") ||
		!tp.is("Static", "True") {
		return refuse("Effect untap denial trigger %q shape", tr)
	}
	if left := tp.unread(); len(left) > 0 {
		return refuse("Effect untap denial trigger unread %v", left)
	}
	e := t.Effect
	ep := newParams(e.Params)
	ep.consume("DB", "Defined", "Origin", "Destination")
	if e.API != "ChangeZone" || e.Sub != nil || !ep.is("Defined", "Self") || !ep.is("Origin", "Command") || !ep.is("Destination", "Exile") {
		return refuse("Effect untap denial trigger %q body", tr)
	}
	if left := ep.unread(); len(left) > 0 {
		return refuse("Effect untap denial trigger body unread %v", left)
	}
	return nil
}
