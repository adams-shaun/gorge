package kgen

import (
	"strings"
)

func init() {
	triggerMode("ChangesZone", trigChangesZone)
}

// trigChangesZone lowers the Mode$ ChangesZone trigger arm, moved out of lowerTriggered (trigger_reg.go).
func trigChangesZone(lc *lowerCtx, a *trigArm) error {
	t, zone, tp, tr, readF := a.t, a.zone, a.tp, a.tr, a.readF
	var err error
	giftSelf := false
	if raw := strings.TrimSpace(tp.str("Origin")); strings.Contains(raw, ",") {
		// an Origin$ set on a trigger: gorge matches the move's from-zone
		// against the set (trigmatch ZoneChangeMatchesWithCapture). Only
		// the "put into a graveyard from anywhere other than the
		// battlefield" shape lowers, onto the kernel's TrigToGraveyard
		// event fired by the placement itself (dirac/zones.go placeFromK).
		if err := lc.trigToGraveyardSet(tp, tr, t, raw, readF); err != nil {
			return err
		}
		return nil
	}
	if tr.Origin, err = zoneOf(tp, "Origin", zAny); err != nil {
		return err
	}
	destMask, destSet, err := czDestSet(tp)
	if err != nil {
		return err
	}
	if destSet {
		tr.Dest = zAny
	} else if tr.Dest, err = zoneOf(tp, "Destination", zAny); err != nil {
		return err
	}
	lc.wasCastOK = tr.Dest == zBattlefield // wasCast reads the entering spell
	tr.F, err = readF("ValidCard")
	lc.wasCastOK = false
	if err != nil {
		return err
	}
	if tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fPromisedGift != 0 {
		// "When this enters, if the gift was promised" (Scrapshooter's
		// ValidCard$ Card.Self+PromisedGift): read on the entering
		// permanent as the event happens (gorge matches ValidCard then)
		if tr.F.Players != 0 || tr.F.Clauses[0] != (lClause{Flags: fSelf | fPromisedGift}) || !lc.out.Gift {
			return refuse("gift trigger filter %s", tr.F)
		}
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		giftSelf = true
		a.giftSelf = true
	}
	switch {
	case destSet:
		ok, err := czPair(tr, destMask, zone, giftSelf, czCostBody(t))
		if err != nil {
			return err
		}
		if !ok {
			return refuse("ChangesZone %s->set", zoneLabel[tr.Origin])
		}
	case tr.Dest == zBattlefield && tr.Origin == zAny:
		tr.Event = "ETB"
	case tr.Origin == zBattlefield && tr.Dest == zGraveyard:
		lowerDies(tp, t, tr)
	case tr.Origin == zBattlefield && tr.Dest == zAny && !giftSelf:
		// leaves the battlefield to any zone (CR 603.6c, 603.10a): the
		// kernel's TrigLeaves fires in fireLeave, the permanent still on
		// the battlefield (gorge changesZoneMatches on the LKI)
		tr.Event = "Leaves"
	default:
		// a destination set on a battlefield leave, a graveyard leave and an
		// Exile origin on an enter (lower_changezonepairs.go)
		ok, err := czPair(tr, destMask, zone, giftSelf, czCostBody(t))
		if err != nil {
			return err
		}
		if !ok {
			return refuse("ChangesZone %s->%s", zoneLabel[tr.Origin], zoneLabel[tr.Dest])
		}
	}
	if tr.Origin == zBattlefield {
		if err := departedFilterRefusal(tr.F); err != nil {
			return err
		}
	}
	if tc, ok := tp.get("TriggerController"); ok && tc != "TriggeredCardController" {
		return refuse("TriggerController %q", tc)
	}
	return nil
}
