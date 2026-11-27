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
// trigger's player refuses.
func (lc *lowerCtx) playTriggeredPlayerHand(valid string) (lRef, string, bool, error) {
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
