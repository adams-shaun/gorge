package kgen

import (
	"fmt"
	"strconv"
	"strings"
)

// deliriumCond is CR 207.2c delirium (gorge graveyardCardTypeCount >= 4):
// four or more card types among the cards in the controller's graveyard.
func deliriumCond() lCond {
	return lCond{Kind: cValue, A: lValue{Kind: vGraveTypes}, Op: opGE, B: cst(4)}
}

// deliriumValue lowers Count$Delirium.<yes>.<no> (gorge cov3BranchHolds):
// <yes> with delirium, else <no>; each a literal or an SVar.
func (lc *lowerCtx) deliriumValue(arms string, depth int) (lValue, error) {
	parts := strings.Split(arms, ".")
	if len(parts) != 2 {
		return lValue{}, fmt.Errorf("Count$Delirium.%s", arms)
	}
	v := lValue{Kind: vCompare, N: 4, Op: opGE, Sub: []lValue{{Kind: vGraveTypes}, {Kind: vBranches}}}
	for _, t := range parts {
		if n, err := strconv.Atoi(t); err == nil {
			v.Sub[1].Sub = append(v.Sub[1].Sub, cst(n))
			continue
		}
		sb, ok := lc.face.SVars[t]
		if !ok {
			return lValue{}, refuse("Count$Delirium branch %q", t)
		}
		sv, err := lc.svarValue(sb, depth+1)
		if err != nil {
			return lValue{}, err
		}
		v.Sub[1].Sub = append(v.Sub[1].Sub, sv)
	}
	return v, nil
}
