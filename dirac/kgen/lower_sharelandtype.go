package kgen

import (
	"slices"
	"strings"
)

// shareLandType reads a library search's ShareLandType$: every found card
// must share a land type with the others (gorge rules.validateSearch rejects
// an answer that does not, through effects.SharedLandTypes; the kernel offers
// a later pick only the cards sharing a type with every earlier one,
// dirac/zone_sharelandtype.go). The kernel compares printed subtypes, which
// are land types exactly when every card the filter admits is a basic land,
// so only a basic-land filter lowers; the other set-level riders do not
// co-occur in the corpus and refuse.
func shareLandType(ps *params, f *lFilter, flags []string) ([]string, error) {
	v, ok := ps.get("ShareLandType")
	if !ok {
		return flags, nil
	}
	if strings.TrimSpace(v) != "True" {
		return nil, refuse("ShareLandType$ %q", v)
	}
	if f.Each || len(f.Clauses) == 0 {
		return nil, refuse("ShareLandType$ on a list pick")
	}
	for _, c := range f.Clauses {
		if c.Types != tLand || c.Flags&fBasic == 0 {
			return nil, refuse("ShareLandType$ with a filter that admits a nonbasic card")
		}
	}
	for _, fl := range flags {
		if fl == "DifferentNames" || strings.HasPrefix(fl, "alt=") {
			return nil, refuse("ShareLandType$ with %s", fl)
		}
	}
	if slices.Contains(flags, "Mandatory") {
		return nil, refuse("ShareLandType$ on a search that must find")
	}
	return append(flags, "ShareLandType"), nil
}
