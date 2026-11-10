package kgen

import (
	"strings"
)

// maxEachGroups bounds the listed types of an EACH pick: the kernel reads a
// card's group as its position in the filter's Or chain and keeps the picked
// groups in one byte.
const maxEachGroups = 8

// pickFilter reads a ChangeZone ChangeType$ for a pick. An "EACH A & B" list
// (Forge's one pick per listed type; gorge eachAlternatives) becomes one
// clause per listed type, in order, with lFilter.Each set: the kernel's pick
// then offers one card per type, a card matching two listed types belonging to
// the first (gorge EachTypeGroups). Any other spec is lc.filter's.
func (lc *lowerCtx) pickFilter(ct string) (*lFilter, error) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(ct), "EACH ")
	if !ok {
		return lc.filter(ct)
	}
	parts := strings.Split(rest, "&")
	if len(parts) < 2 || len(parts) > maxEachGroups {
		return nil, refuse("EACH list of %d types", len(parts))
	}
	out := &lFilter{Each: true}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, refuse("EACH list with an empty entry")
		}
		sub, err := lc.filter(p)
		if err != nil {
			return nil, err
		}
		if len(sub.Clauses) != 1 || sub.Players != 0 {
			return nil, refuse("EACH entry %q is not one object clause", p)
		}
		out.Clauses = append(out.Clauses, sub.Clauses[0])
	}
	return out, nil
}

// eachPickGate refuses the EACH shapes the kernel does not read: only a
// one-card-per-type pick out of the library (to the hand, the battlefield or
// the graveyard) or out of the hand onto the battlefield (the hand pick is
// checked further in handMove, which owns Optional$).
func eachPickGate(ps *params, origin, dest int, n lValue) error {
	if !isConstN(n, 1) {
		return refuse("EACH pick of %v cards per type", n)
	}
	switch {
	case origin == zLibrary && (dest == zHand || dest == zBattlefield || dest == zGraveyard):
	case origin == zHand && dest == zBattlefield:
	default:
		return refuse("EACH pick %s->%s", zoneLabel[origin], zoneLabel[dest])
	}
	for _, k := range []string{"DifferentNames", "Shuffle", "NoShuffle", "OriginAlternative", "RememberChanged", "ForgetChanged", "AtRandom", "WithTotalCMC", "WithDifferentPowers"} {
		if _, ok := ps.m[k]; ok && !(k == "Shuffle" && ps.is("Shuffle", "True")) {
			return refuse("EACH pick with %s$", k)
		}
	}
	if _, ok := ps.m["ValidTgts"]; ok {
		return refuse("EACH pick with ValidTgts$")
	}
	return nil
}
