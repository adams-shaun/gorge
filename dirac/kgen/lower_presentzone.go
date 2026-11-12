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
	if !ok || z == zNone {
		return c, refuse("PresentZone %q", zones)
	}
	return lCond{Kind: cSourceZone, Zone: uint8(z)}, nil
}

// triggeredPresentZoneCond lowers a trigger's PresentZone$ beside its
// IsPresent$ spec ip. A source-identity read (Card.StrictlySelf) is an
// intervening-if kept and re-checked at resolution (CR 603.4); every other
// spec gets the static present-zone count.
func (lc *lowerCtx) triggeredPresentZoneCond(c lCond, tp *params, ip string) (lCond, error) {
	pz, ok := tp.get("PresentZone")
	if !ok {
		return c, nil
	}
	if _, cmp := tp.get("PresentCompare"); !cmp && strings.TrimSpace(ip) == "Card.StrictlySelf" {
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

// presentZoneCond rewrites a static's IsPresent$ condition c for its
// PresentZone$ (gorge countStaticPresent: the matching objects summed over
// the named zones; a non-battlefield zone reads a bare Permanent as a
// permanent card, presentSpecForZone, which the kernel's card matcher
// already does): Battlefield is c itself; Graveyard / Exile count the
// cards there (every graveyard / exile zone, the filter's owner terms
// narrowing); a list sums. Hand, Library, Stack and Command refuse.
func presentZoneCond(c lCond, zones, spec string, lc *lowerCtx) (lCond, error) {
	c0 := c
	var sum lValue
	for i, z := range strings.Split(zones, ",") {
		f, err := lc.filter(spec)
		if err != nil {
			return c, err
		}
		var v lValue
		zn := strings.TrimSpace(z)
		if zn != "Battlefield" {
			// the kernel's card matcher (graveyard / exile counts) has no
			// source: a Self or Other term there would match any card
			for _, c := range f.Clauses {
				if c.Flags&(fSelf|fOther) != 0 {
					return c0, refuse("PresentZone %q of a source-relative filter", zones)
				}
			}
		}
		switch zn {
		case "Battlefield":
			v = lValue{Kind: vCount, F: f}
		case "Graveyard":
			v = lValue{Kind: vCountGraveyard, F: f}
		case "Exile":
			v = lValue{Kind: vCountExile, F: f}
		default:
			return c, refuse("PresentZone %q", zones)
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
