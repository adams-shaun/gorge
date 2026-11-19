package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// remRestBranches are the params whose value names the SVar of a chain
// branch that runs inside the same resolution: Clash's win and lose arms
// (gorge effClash runs the named sub ability before the resolution ends) and
// a delayed trigger's body (Mobilize's RememberChain$ False group program).
var remRestBranches = []string{"WinSubAbility", "LoseSubAbility", "Execute"}

// remRestReads reports whether the chain after n (its Sub links and the
// branches it names) reads the remembered set: a param value naming
// Remembered, or naming a card SVar whose text does (Pyretic Rebirth's NumDmg$
// X with SVar X:Remembered$CardManaCost; Gilt-Leaf Ambush's Clash
// WinSubAbility$ PumpAll ValidCards$ Creature.IsRemembered). A writer whose
// set is read only this way is read inside its own resolution, so the
// kernel's per-resolution set is gorge's source-held one for that read;
// a read by another ability is refuseCrossAbilityRemembered's.
func (lc *lowerCtx) remRestReads(n *cards.SA) bool {
	seen := map[*cards.SA]bool{}
	var walk func(n *cards.SA, depth int) bool
	walk = func(n *cards.SA, depth int) bool {
		for ; n != nil && !seen[n] && depth < 16; n = n.Sub {
			seen[n] = true
			for k, v := range n.Params {
				if k == "SubAbility" {
					continue
				}
				if lc.remValueNamesRemembered(v) {
					return true
				}
			}
			for _, k := range remRestBranches {
				if k == "Execute" && n.API != "DelayedTrigger" {
					continue
				}
				if b := cards.ResolveSVar(lc.face.SVars, n.Params[k]); b != nil && walk(b, depth+1) {
					return true
				}
			}
		}
		return false
	}
	return walk(n, 0)
}

// remValueNamesRemembered: v names Remembered, or a card SVar whose text
// does (one level: an amount SVar such as X:Remembered$CardManaCost).
func (lc *lowerCtx) remValueNamesRemembered(v string) bool {
	if strings.Contains(v, "Remembered") {
		return true
	}
	for _, tok := range strings.FieldsFunc(v, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) {
		sv, ok := lc.face.SVars[tok]
		if !ok || strings.Contains(sv, "$") && strings.Contains(sv, "DB$") {
			continue // an ability SVar is a branch, walked by its own param
		}
		if strings.Contains(sv, "Remembered") {
			return true
		}
	}
	return false
}
