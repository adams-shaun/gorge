package kgen

import "strings"

// playTriggeredPlayerHand lowers a Play whose Valid$ names the trigger's
// player by a control predicate over ValidZone$ Hand: Mindleech Mass's
// Card.ControlledBy TriggeredTarget and Silent-Blade Oni's
// Card.nonLand+OwnedBy TriggeredTarget, the player a DamageDone trigger hit.
// gorge effPlay walks only that referent's hand (playHandSeats), so the
// population is RefTriggerPlayer's hand and the control predicate drops: the
// ref picks the seat. The remaining Valid$ qualifiers become the filter. It
// reports false for any other spelling; a referent that is not the
// trigger's player refuses. emit builds the CastFree over the hand.
func (lc *lowerCtx) playTriggeredPlayerHand(valid, vzone string, forget bool, saExpr string, emit func(lRef, int, string) error) (bool, error) {
	obj, filter, ok, err := lc.playTriggeredPlayerHandShape(valid)
	if !ok || vzone != "Hand" || forget {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	filter, err = andFilterExpr(filter, saExpr)
	if err != nil {
		return true, err
	}
	return true, emit(obj, zHand, filter)
}

// playTriggeredPlayerHandShape matches a Card.<qualifier>*ControlledBy|OwnedBy
// TriggeredTarget spelling and returns the trigger-player referent plus the
// remaining qualifiers as a card filter.
func (lc *lowerCtx) playTriggeredPlayerHandShape(valid string) (lRef, string, bool, error) {
	quals, ok := strings.CutPrefix(valid, "Card.")
	if !ok || strings.ContainsAny(quals, ",.") {
		return lRef{}, "", false, nil
	}
	var kept []string
	ctl := 0
	for part := range strings.SplitSeq(quals, "+") {
		switch part {
		case "ControlledBy TriggeredTarget", "OwnedBy TriggeredTarget":
			ctl++
		default:
			kept = append(kept, part)
		}
	}
	if ctl != 1 {
		return lRef{}, "", false, nil
	}
	obj, err := lc.defined("TriggeredTarget")
	if err != nil {
		return lRef{}, "", true, err
	}
	if obj.Kind != rTriggeredPlayer {
		return lRef{}, "", true, refuse("Play Valid$ %q: the referent is not the trigger's player", valid)
	}
	if len(kept) == 0 {
		return obj, "", true, nil
	}
	return obj, "Card." + strings.Join(kept, "+"), true, nil
}
