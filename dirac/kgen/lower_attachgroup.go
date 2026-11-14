package kgen

import "strings"

// attachGroupPrefix opens the dotted `AttachedTo Targeted.<types>` selector
// (gorge attachedToDefinedSelector): the Auras / Equipment attached to the
// chain's target, read at resolution time and, once the target has left the
// battlefield, from the attachments its departure left lingering (the
// were-attached half: LastBearer). Fumble spells it in both halves of its
// chain -- "gain control of all Auras and Equipment that were attached to
// it, then attach them to another creature".
const attachGroupPrefix = "AttachedTo Targeted."

// noteRememberAttachedGroup marks the ExtRememberAll a GainControl over
// attachGroupPrefix emits (kremWrites sees "remember"); the Attach after it
// reads that set, so attachGroupChoices checks for the mark.
const noteRememberAttachedGroup = "remember attached group"

// attachedGroupFilter reads the qualifier list after attachGroupPrefix (a
// comma-OR of object words, as attachedToDefinedSelector reads it) as the
// kernel filter "<word>.AttachedTo Targeted" per word: F2AttachedToT0 over
// target 0 of the entry, which the lingering-attachment read extends to a
// target that has left the battlefield. The chain must declare the one
// object target the selector names, as destroyAllFilter requires.
func (lc *lowerCtx) attachedGroupFilter(spec string) (*lFilter, error) {
	quals, ok := strings.CutPrefix(spec, attachGroupPrefix)
	if !ok || strings.TrimSpace(quals) == "" {
		return nil, refuse("%q: not an AttachedTo Targeted group", spec)
	}
	if lc.ab == nil || len(lc.ab.Targets) != 1 || lc.ab.Targets[0].F == nil ||
		lc.ab.Targets[0].F.Players != 0 || len(lc.ab.Targets[0].F.Clauses) == 0 || lc.ab.Targets[0].Zone != zBattlefield {
		return nil, refuse("%q without one battlefield object target slot upstream", spec)
	}
	var alts []string
	for _, w := range strings.Split(quals, ",") {
		w = strings.TrimSpace(w)
		if w == "" || strings.ContainsAny(w, ".+!$ ") {
			return nil, refuse("%q: qualifier %q", spec, w)
		}
		alts = append(alts, w+".AttachedTo Targeted")
	}
	lc.attachedTgtOK = true
	defer func() { lc.attachedTgtOK = false }()
	return lc.filter(strings.Join(alts, ","))
}

// gainControlAttached lowers `DB$ GainControl | Defined$ AttachedTo
// Targeted.<types> | NewController$ You`: the group joins the remembered set
// (ExtRememberAll, after a ClearRemembered so nothing else is in it) and the
// GainControl takes every member (kernel opGainControl over RefRemembered).
// Only a permanent, resolving-controller gain lowers; LoseControl$, Untap$
// and the other riders are left unread and refuse.
func (lc *lowerCtx) gainControlAttached(ps *params, spec string) error {
	if ps.str("NewController") != "You" {
		return refuse("GainControl %q with NewController$ %q", spec, ps.str("NewController"))
	}
	if ps.peek("ValidTgts") != "" {
		return refuse("GainControl %q beside the node's own ValidTgts$", spec)
	}
	f, err := lc.attachedGroupFilter(spec)
	if err != nil {
		return err
	}
	ps.consume("Defined", "NewController")
	lc.emit(lInstr{Op: "ClearRemembered"})
	lc.emit(lInstr{Op: "RememberAll", F: f, CS: "ObjOrder", Note: noteRememberAttachedGroup})
	lc.emit(lInstr{Op: "GainControl", Obj: lRef{Kind: rRemembered}, CS: "dur=Permanent"})
	return nil
}

// attachGroupChoices lowers `DB$ Attach | Object$ AttachedTo Targeted.<types>
// | Choices$ <filter>` (gorge effAttach's destination-side pool): the
// resolving controller picks one permanent the filter admits and every object
// of the group the GainControl before it remembered is attached to it. The
// ChooseCard carries chooseAttachDest; the AttachObj after it reads the pick
// (RefChosen) and the set (RefRemembered). Optional$, Move$ and the other
// riders refuse. The group is the one the same chain's GainControl gathered.
func (lc *lowerCtx) attachGroupChoices(ps *params, spec string) error {
	for _, k := range []string{"Optional", "Move", "RememberAttached", "PlayerChoices", "Unattach", "ChoiceZone", "Chooser", "ValidTgts", "Defined"} {
		if ps.peek(k) != "" {
			return refuse("Attach Object %q with %s$", spec, k)
		}
	}
	gathered := false
	for _, in := range lc.ab.Prog {
		if in.Op == "RememberAll" && in.Note == noteRememberAttachedGroup {
			gathered = true
		}
	}
	if !gathered {
		return refuse("Attach Object %q without the GainControl that gathers the group", spec)
	}
	f, err := lc.filter(ps.str("Choices"))
	if err != nil {
		return err
	}
	ps.consume("Object", "Choices", "ChoiceTitle")
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zBattlefield, CS: "AttachDest"})
	lc.emit(lInstr{Op: "AttachObj", Obj: lRef{Kind: rChosen}, Obj2: lRef{Kind: rRemembered}})
	lc.emit(lInstr{Op: "ClearRemembered"})
	return nil
}
