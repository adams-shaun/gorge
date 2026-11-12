package kgen

import (
	"strconv"
	"strings"
)

// Count$wasCastFromExile.<yes>.<no> (gorge effects/count_body_heads.go
// evalCountBodyDottedWasCastFromExile; Delayed Blast Fireball's 5 instead of
// 2): <yes> when the resolving spell was cast from exile (a foretold cast, a
// warp recast, a may-play cast), else <no>; a copy was never cast. It reads
// the spell's own stack entry, so only a spell ability has the same object.
// It lowers to a Count$Compare over the spell's own PredWasCastSpell origin
// read (the Card.wasCastFromExile filter, spellExile): the object counts 1
// when it was cast from exile, 0 otherwise.
func (lc *lowerCtx) castFromExileValue(arms string) (lValue, error) {
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return lValue{}, refuse("Count$wasCastFromExile outside a spell ability")
	}
	parts := strings.Split(arms, ".")
	if len(parts) != 2 {
		return lValue{}, refuse("Count$wasCastFromExile arms %q", arms)
	}
	var br [2]lValue
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return lValue{}, refuse("Count$wasCastFromExile arms %q", arms)
		}
		br[i] = cst(n)
	}
	save := lc.spellProvOK
	lc.spellProvOK = true
	f, err := lc.filter("Card.wasCastFromExile")
	lc.spellProvOK = save
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vCompare, N: 1, Op: opGE, Sub: []lValue{{Kind: vRefCount, Ref: lRef{Kind: rSelf}, F: f}, {Kind: vBranches, Sub: br[:]}}}, nil
}
