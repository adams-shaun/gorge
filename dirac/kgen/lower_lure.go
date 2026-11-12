package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lureGrant lowers a Continuous static's AddHiddenKeyword$ "All creatures able
// to block CARDNAME do so." (CR 509.1c; gorge kw:AllMustBlock, a layer-6
// keyword grant read per attacker by combat.DerivedHiddenFlags): a StatLure
// over the static's Affected$ filter, under the static's condition. It is
// the whole static (lureOnly: Lure's, Seton's Desire's and Stone-Tongue
// Basilisk's) or rides beside other parts (Gift of the Deity's +1/+1), which
// then lower as the ordinary Continuous static and the lure follows as a
// second one. Any other AddHiddenKeyword$ sentence is left to continuous.
func (lc *lowerCtx) lureGrant(sp *params, st *lStatic) (lure lStatic, lureOnly bool, err error) {
	hk, ok := sp.m["AddHiddenKeyword"]
	if !ok || cards.HiddenKeywordOf(strings.TrimSpace(hk)) != cards.HiddenKeywordAllMustBlock {
		return lStatic{}, false, nil
	}
	aff := sp.str("Affected")
	if aff == "" {
		return lStatic{}, false, refuse("AddHiddenKeyword %q without Affected$", hk)
	}
	f, err := lc.filter(aff)
	if err != nil {
		return lStatic{}, false, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return lStatic{}, false, refuse("AddHiddenKeyword %q over a player filter", hk)
	}
	sp.consume("AddHiddenKeyword")
	lure = lStatic{Kind: "Lure", F: f, Cond: st.Cond}
	return lure, len(sp.unread()) == 0, nil
}
