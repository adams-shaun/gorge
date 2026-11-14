package kgen

import (
	"strings"
)

// selfPresentZoneCond lowers IsPresent$ Card.StrictlySelf beside a single
// PresentZone$ zone. The read is gorge's intervening-if, checked when the
// event happens and again when the trigger resolves (CR 603.4), so it cannot
// be dropped: it is the source's live zone (cSourceZone), not the ordinary
// cPresent source count. The ordinary count reads the source slot only while
// the object's version equals the trigger's captured version (ctx.srcSl in
// exprs.go), so a source that leaves and returns before resolution would read
// as absent; gorge's Self is the stable object id, which survives a zone
// change, and remains present. A zone list or a non-default PresentCompare$
// refuses; the carriers name one zone with the default compare (Arden Angel
// and the graveyard family, Inalla/Arahbo/Edgar Markov/Sidar Jabari's Command
// and Battlefield twins).
func selfPresentZoneCond(c lCond, zones string) (lCond, error) {
	zs := strings.Split(zones, ",")
	if len(zs) != 1 {
		return c, refuse("PresentZone %q of a source-identity presence read", zones)
	}
	z, ok := zoneNames[strings.TrimSpace(zs[0])]
	if !ok || z == zNone || z == zAny {
		// Any/All (both zAny) must fail closed: gorge's presentZoneFromParam
		// rejects them, while zAny's kernel zone is ZoneNone, which
		// CondSourceZone reads true for an object in no zone.
		return c, refuse("PresentZone %q", zones)
	}
	return lCond{Kind: cSourceZone, Zone: uint8(z)}, nil
}

// triggeredPresentZoneCond lowers a trigger's PresentZone$ beside its
// IsPresent$ spec ip. A source-identity read (Card.StrictlySelf) is an
// intervening-if kept and re-checked at resolution (CR 603.4); every other
// spec gets the static present-zone count, scoped by its PresentPlayer$
// (presentPlayerCond).
func (lc *lowerCtx) triggeredPresentZoneCond(c lCond, tp *params, ip string) (lCond, error) {
	pz, ok := tp.get("PresentZone")
	_, cmp := tp.get("PresentCompare")
	if !ok || cmp || strings.TrimSpace(ip) != "Card.StrictlySelf" {
		if pp, has := tp.get("PresentPlayer"); has {
			if _, pd := tp.get("PresentDefined"); pd {
				// gorge presentClauseHolds counts a PresentDefined$ group
				// unscoped, PresentPlayer$ unread
				return c, refuse("trigger PresentPlayer$ beside PresentDefined$")
			}
			if !ok {
				pz = "Battlefield"
			}
			return lc.presentPlayerCond(c, pp, pz, ip)
		}
	}
	if !ok {
		return c, nil
	}
	if !cmp && strings.TrimSpace(ip) == "Card.StrictlySelf" {
		// PresentPlayer$ You names the zone the source card lies in (its
		// owner's), which is the reading player for its own
		// graveyard/command-zone trigger.
		if pp, ok := tp.get("PresentPlayer"); ok && strings.TrimSpace(pp) != "You" {
			return c, refuse("PresentPlayer %q of a source-identity presence read", pp)
		}
		return selfPresentZoneCond(c, pz)
	}
	return presentZoneCond(c, pz, ip, lc)
}

// presentPlayerCond lowers a trigger's PresentPlayer$ beside its IsPresent$
// spec ip and zone list pz (gorge presentClauseHolds, Forge's CardTraitBase
// readPresentPlayer): Any counts every seat's copies, the plain zone count;
// You counts only the trigger controller's, which is the spec narrowed per
// zone to permanents it controls (Battlefield, gorge
// presentControllerInScope) or to cards it owns (a graveyard or exile zone is
// its owner's, CR 400.3; Pit Keeper, the Desert family's Battlefield,
// Graveyard). Opponent and any other word refuse.
func (lc *lowerCtx) presentPlayerCond(c lCond, pp, pz, ip string) (lCond, error) {
	switch strings.TrimSpace(pp) {
	case "Any":
		return presentZoneCond(c, pz, ip, lc)
	case "You":
	default:
		return c, refuse("trigger PresentPlayer$ %q", pp)
	}
	scoped := func(zone string) string {
		if zone == "Battlefield" {
			return addFilterProp(ip, "YouCtrl")
		}
		return addFilterProp(ip, "YouOwn")
	}
	if strings.TrimSpace(pz) == "Battlefield" {
		f, err := lc.filter(scoped("Battlefield"))
		if err != nil {
			return c, err
		}
		c.F = f
		return c, nil
	}
	var sum lValue
	for i, z := range strings.Split(pz, ",") {
		zn := strings.TrimSpace(z)
		v, err := presentZoneValue(zn, pz, scoped(zn), lc)
		if err != nil {
			return c, err
		}
		if i == 0 {
			sum = v
			continue
		}
		sum = lValue{Kind: vSum, Sub: []lValue{sum, v}}
	}
	return lCond{Kind: cValue, A: sum, Op: c.Op, B: c.A}, nil
}

// presentZoneCond rewrites a static's IsPresent$ condition c for its
// PresentZone$ (gorge countStaticPresent: the matching objects summed over
// the named zones; a non-battlefield zone reads a bare Permanent as a
// permanent card, presentSpecForZone, which the kernel's card matcher
// already does): Battlefield is c itself; Graveyard / Exile count the
// cards there (every graveyard / exile zone, the filter's owner terms
// narrowing); a list sums. Hand, Library, Stack and Command refuse.
func presentZoneCond(c lCond, zones, spec string, lc *lowerCtx) (lCond, error) {
	var sum lValue
	for i, z := range strings.Split(zones, ",") {
		v, err := presentZoneValue(strings.TrimSpace(z), zones, spec, lc)
		if err != nil {
			return c, err
		}
		if i == 0 {
			sum = v
			continue
		}
		sum = lValue{Kind: vSum, Sub: []lValue{sum, v}}
	}
	if zones == "Battlefield" {
		return c, nil
	}
	return lCond{Kind: cValue, A: sum, Op: c.Op, B: c.A}, nil
}

// presentZoneValue is the count of spec's matches in one present zone zn of
// the PresentZone$ list zones (presentZoneCond's per-zone term).
func presentZoneValue(zn, zones, spec string, lc *lowerCtx) (lValue, error) {
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	if zn != "Battlefield" {
		// the kernel's card matcher (graveyard / exile counts) has no
		// source: a Self or Other term there would match any card
		for _, c := range f.Clauses {
			if c.Flags&(fSelf|fOther) != 0 {
				return lValue{}, refuse("PresentZone %q of a source-relative filter", zones)
			}
		}
	}
	switch zn {
	case "Battlefield":
		return lValue{Kind: vCount, F: f}, nil
	case "Graveyard":
		return lValue{Kind: vCountGraveyard, F: f}, nil
	case "Exile":
		return lValue{Kind: vCountExile, F: f}, nil
	}
	return lValue{}, refuse("PresentZone %q", zones)
}
