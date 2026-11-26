package kgen

import (
	"strings"
)

// The bodies of the trigger statics (static_mode.go).
func init() {
	staticMode("Panharmonicon", (*staticCtx).modePanharmonicon)
	staticMode("DisableTriggers", (*staticCtx).modeDisableTriggers)
}

func (sc *staticCtx) modePanharmonicon() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	var err error
	// gorge panharmoniconEchoes: a battlefield static whose ValidCard$
	// matches the triggering ability's source adds one trigger
	// placement. The one other shape read is the entering-permanent
	// family: ValidMode$ ChangesZone,ChangesZoneAll
	// with Destination$ Battlefield and a ValidCause$ filter, which also
	// needs the cause (the entering permanent or token) to match F2.
	// A ValidMode$ of one player-event mode alone (LifeGained, Drawn),
	// with an optional ValidPlayer$ You, echoes only that mode's triggers
	// of that player (flags "LifeGained" / "Drawn" and "You").
	// The same ValidMode$ with Destination$ Graveyard and Origin$ Battlefield
	// is the dies form (flag "Dies"): a ChangesZone trigger caused by a
	// matching permanent dying.
	// Every other gate (Origin$, Condition$, ...) is unread, so the shared
	// unread check refuses it by name.
	v, ok := sp.get("ValidCard")
	if !ok {
		return nil, true, refuse("Panharmonicon without ValidCard$")
	}
	if sc.st.Cond.Kind != cNone && sc.st.Cond.Kind != cEnduringStory {
		// the EnduringStory gate lowers (staticGateCond, gorge
		// panharmoniconEchoes' continuousGateHolds read: Bifur, Melodic
		// Rider, the corpus's only gated Panharmonicon static); any other
		// Condition$ / IsPresent$ / CheckSVar$ gate is unread, so the shared
		// unread check refuses it by name.
		return nil, true, refuse("Panharmonicon with a Condition$ / IsPresent$ / CheckSVar$ gate")
	}
	if sc.st.F, err = lc.filter(v); err != nil {
		return nil, true, err
	}
	vm, hasMode := sp.get("ValidMode")
	cause, hasCause := sp.get("ValidCause")
	dest, hasDest := sp.get("Destination")
	switch {
	case !hasMode && !hasCause && !hasDest:
	case hasMode && !hasCause && !hasDest && (vm == "LifeGained" || vm == "Drawn"):
		sc.st.Flags = append(sc.st.Flags, vm)
		switch vp, ok := sp.get("ValidPlayer"); {
		case !ok:
		case strings.TrimSpace(vp) == "You":
			sc.st.Flags = append(sc.st.Flags, "You")
		default:
			return nil, true, refuse("Panharmonicon ValidPlayer$ %s", vp)
		}
	case hasMode && hasCause && hasDest:
		if vm != "ChangesZone,ChangesZoneAll" {
			return nil, true, refuse("Panharmonicon ValidMode$ %s", vm)
		}
		switch dest {
		case "Battlefield":
		case "Graveyard":
			// the dies form: a creature's battlefield-to-graveyard move
			// (kop.PanhModeDies, Filter2 = the cause)
			if org, _ := sp.get("Origin"); org != "Battlefield" {
				return nil, true, refuse("Panharmonicon Origin$ %s", org)
			}
			sc.st.Flags = append(sc.st.Flags, "Dies")
		default:
			return nil, true, refuse("Panharmonicon Destination$ %s", dest)
		}
		if sc.st.F2, err = lc.filter(cause); err != nil {
			return nil, true, err
		}
	default:
		return nil, true, refuse("Panharmonicon with a partial ValidMode$ / ValidCause$ / Destination$ set")
	}
	return nil, false, nil
}

func (sc *staticCtx) modeDisableTriggers() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.disableTriggersStatic(sp, &sc.st); err != nil {
		return nil, true, err // lower_disabletriggers.go
	}
	return nil, false, nil
}
