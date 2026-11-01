package kgen

import (
	"strings"
)

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
