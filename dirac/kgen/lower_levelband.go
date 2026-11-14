package kgen

import "strings"

// levelband: a Level up band's gate (CR 702.87, the LEVEL N-M lines),
//
//	IsPresent$ Card.Self+counters_GE<a>_LEVEL+counters_LT<b>_LEVEL
//
// holds exactly while the source has a LEVEL count in [a, b). The kernel
// filter carries one comparison per clause, so the gate lowers as the AND of
// one single-compare Card.Self condition per counter test: the source is the
// only object Card.Self names, so "a present object satisfies both" and
// "the source satisfies each" are the same thing. A band with one compare
// (counters_GE3_LEVEL) is the ordinary filter and is not split here.

// levelBandFilters splits an IsPresent$ value of the shape
// Card.Self+counters_<cmp>_LEVEL(+counters_<cmp>_LEVEL)+ into one
// Card.Self+counters_<cmp>_LEVEL filter per compare, or returns nil for any
// other shape (including a single compare).
func levelBandFilters(ip string) []string {
	parts := strings.Split(strings.TrimSpace(ip), "+")
	if len(parts) < 3 || parts[0] != "Card.Self" {
		return nil
	}
	out := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		if !strings.HasPrefix(p, "counters_") || !strings.HasSuffix(p, "_LEVEL") {
			return nil
		}
		out = append(out, "Card.Self+"+p)
	}
	return out
}

// levelBandCond reports whether c is a Level up band's gate: a conjunction
// of Card.Self single-compare LEVEL conditions (what staticBody builds from
// an IsPresent$ of levelBandFilters, or from the one-compare form).
func levelBandCond(c lCond) bool {
	switch c.Kind {
	case cAnd:
		for _, s := range c.Sub {
			if !levelBandCond(s) {
				return false
			}
		}
		return len(c.Sub) > 0
	case cPresent:
		if c.Neg || c.Op != opGE || c.A.Kind != vConst || c.A.N != 1 || c.F == nil || c.F.Players != 0 || len(c.F.Clauses) != 1 {
			return false
		}
		cl := c.F.Clauses[0]
		return cl.Flags == fSelf && cl.CmpStat == cmpCounters && cl.CmpCounter == "LEVEL" && cl.Types == 0 && cl.TypesAll == 0 &&
			cl.NotTypes == 0 && cl.Subtype == "" && cl.NotSubtype == "" && cl.Colors == "" && cl.NotColors == "" && cl.Ctrl == 0 && cl.Owner == 0
	}
	return false
}

// levelGrantStatic lowers a level band's AddStaticAbility$ self grant (CR
// 702.87, 613.1f): gorge's layer walk (layers.go) queues the inner static
// while the outer band static is live, so it lowers as the inner static
// ANDed with the band's own gate (gatedGrantStatic, the Station template).
func (lc *lowerCtx) levelGrantStatic(sp *params, name string, band lCond) error {
	if _, ok := sp.m["AddReplacementEffect"]; ok {
		return refuse("level band static grants AddReplacementEffect$ beside AddStaticAbility$")
	}
	if _, ok := sp.m["AddSVar"]; ok && !sp.used["AddSVar"] {
		return refuse("level band static grant with AddSVar$")
	}
	return lc.gatedGrantStatic(sp, name, band, "level band", true)
}
