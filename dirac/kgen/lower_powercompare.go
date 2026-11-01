package kgen

import "strings"

// power LT/GT compares.
//
// A literal "powerLT6" / "powerGT3" is exact on integers: LT n is LE n-1 and
// GT n is GE n+1 (gorge numericPred compares the current power or toughness
// with the integer). intCmpGELE folds the strict forms into the two the
// kernel has; every other operator comes back unchanged, so the caller's
// default arm still refuses it.
func intCmpGELE(op uint8, n int) (uint8, int) {
	switch op {
	case opLT:
		return opLE, n - 1
	case opGT:
		return opGE, n + 1
	}
	return op, n
}

// pwVsSrcSplit lifts the one "powerLTX" / "powerGTX" conjunct out of a
// CantBlockBy static's ValidBlocker$ ("creatures with power less than this
// creature's power can't block it"). X must be an SVar whose body is exactly
// Count$CardPower: gorge resolves it through the static's own SVars with the
// static's host as the card (MatchesStaticSpec -> staticSpecCtx), so X is the
// host's current power. The rest of the clause is returned for the ordinary
// filter lowering and mode is kop.Static.PwVsSrc (1 = blocker power < the
// host's, 2 = >). A clause with no such conjunct comes back unchanged with
// mode 0; one this cannot read exactly (alternatives, two compares, another
// SVar body) also comes back unchanged and the filter lowering refuses it
// ("power compare against a value").
func pwVsSrcSplit(expr string, svars map[string]string) (rest string, mode int8) {
	if strings.Contains(expr, ",") {
		return expr, 0
	}
	head, props, ok := strings.Cut(expr, ".")
	if !ok {
		return expr, 0
	}
	parts := strings.Split(props, "+")
	keep := parts[:0:0]
	for _, p := range parts {
		var m int8
		switch {
		case strings.HasPrefix(p, "powerLT"):
			m = 1
		case strings.HasPrefix(p, "powerGT"):
			m = 2
		}
		x := strings.TrimPrefix(strings.TrimPrefix(p, "powerLT"), "powerGT")
		if m == 0 || x == "" || strings.TrimSpace(svars[x]) != "Count$CardPower" {
			keep = append(keep, p)
			continue
		}
		if mode != 0 {
			return expr, 0 // two power compares
		}
		mode = m
	}
	if mode == 0 {
		return expr, 0
	}
	if len(keep) == 0 {
		return head, mode
	}
	return head + "." + strings.Join(keep, "+"), mode
}
