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

// effectUntapDenial lowers a DB$ Effect whose one StaticAbilities$ body is
// gorge's UntapDenial grant (engine/effects effEffect: Affected$ over
// RememberedPlayerCtrl with AddHiddenKeyword$ "This card doesn't untap during
// your next untap step.", registered as a restriction rules' untap-step scan
// reads) beside a Triggers$ line that is the Effect's own end: Mode$ Phase |
// Phase$ Untap | ValidPlayer$ Player.IsRemembered, exiling the Effect from the
// command zone (Exhaustion). The restriction holds the targeted player's
// creatures and lands tapped through that player's next untap step and ends
// there, which is OpSkipUntap C=SkipUntapPlayerCL over the player target (CR
// 611.2b). It reports whether the Effect had that shape; any other
// static-beside-trigger Effect returns false for the pair lowering to read, and
// a half-matching one refuses by name.
func (lc *lowerCtx) effectUntapDenial(ps *params, tr string) (bool, error) {
	list, ok := ps.get("StaticAbilities")
	if !ok {
		return false, nil
	}
	ss, err := lc.effectStaticBodies(list)
	if err != nil {
		return false, nil // the pair lowering names it
	}
	if len(ss) != 1 || ss[0].Mode != "Continuous" ||
		!cards.IsHiddenUntapNextStepKeyword(strings.TrimSpace(ss[0].Params["AddHiddenKeyword"])) {
		delete(ps.used, "StaticAbilities")
		return false, nil
	}
	why := func(more string) (bool, error) {
		return true, refuse("Effect untap denial %q (%s)", list, more)
	}
	if lc.ab.Kind == akMana {
		return why("in a mana ability")
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
	lc.emit(lInstr{Op: "SkipUntap", Obj: obj, C: kop.SkipUntapPlayerCL, Note: "the target player's creatures and lands"})
	// gorge registers the Effect's Phase$ Untap trigger as a delayed trigger
	// that goes on the stack as the target's upkeep begins and exiles the
	// Effect there: the kernel's counterpart is a delayed program with no body
	// (the Effect is no object here), so both sides pose that stack object
	lat := lc.newLatent("DelayedTrigger")
	lc.emit(lInstr{Op: "DelayedTrigger", Obj: obj, C: lat.Index, CS: "NextUpkeepOfPlayer"})
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
	tp := newParams(t.Params)
	tp.consume("Mode", "Phase", "ValidPlayer", "TriggerZones", "Static", "Execute", "TriggerDescription")
	if !tp.is("Phase", "Untap") || !tp.is("ValidPlayer", "Player.IsRemembered") ||
		!tp.is("TriggerZones", "Command") || !tp.is("Static", "True") {
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
