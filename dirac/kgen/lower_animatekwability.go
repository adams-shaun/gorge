package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// animateGrantedKeywordAbilities splits an Animate's Keywords$ value: each
// "Equip:N" word is an activated ability, not a kop.KwMask bit (The
// Irencrag's "it gains equip {3}"), so it lowers as a latent ability the
// override grants (gorge mints it with cards.GrantedKeywordAbility, the
// printed kwEquip expander's body on a scratch face). The latent keeps the
// expander's Name "Equip" (set by attach) so build marks it kop.AfEquip.
// The remaining words return joined for pumpKeywords. Any other
// activated-keyword head stays there and refuses by name.
//
// Only beside the same Animate's RemoveAllAbilities$ (o.LoseAbilities, read
// first): gorge then never offers the granted equip, and the kernel offers
// none either (a granted activation needs an attached host), so the latent
// is never activated in either engine. Without the loss gorge offers it as a
// keyword option the oracle has no canonical form for and the kernel does
// not host (Domesticated Watercourse), so that refuses.
func (lc *lowerCtx) animateGrantedKeywordAbilities(kw string, o *lOverride) (string, error) {
	var rest []string
	for _, w := range strings.Split(kw, "&") {
		w = strings.TrimSpace(w)
		if cards.KeywordHead(w) != "Equip" {
			rest = append(rest, w)
			continue
		}
		if !o.LoseAbilities {
			return "", refuse("Animate Keywords$ %q without RemoveAllAbilities$: a self-granted equip the kernel does not host", w)
		}
		sa := cards.GrantedKeywordAbility(w)
		if sa == nil || sa.Kind != "AB" {
			return "", refuse("keyword %q", w)
		}
		idx, err := lc.grantedKeywordLatent(sa)
		if err != nil {
			return "", err
		}
		o.GrantAbility = append(o.GrantAbility, idx)
	}
	return strings.Join(rest, " & "), nil
}

// grantedKeywordLatent lowers a synthesized keyword ability as a latent
// ability (grantedAbility's shape, minus its "Granted" rename).
func (lc *lowerCtx) grantedKeywordLatent(sa *cards.SA) (int, error) {
	lc.granted++
	defer func() { lc.granted-- }()
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerAbility(sa) }); err != nil {
		return 0, err
	}
	lc.out.Abilities[idx].Latent = true
	return idx, nil
}
