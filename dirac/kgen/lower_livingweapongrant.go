package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
)

// Living weapon granted by a layer-6 AddKeyword$ (Ekthi, Contaminator
// Priest's "each Equipment you control has living weapon"; CR 702.92a,
// 613.1f). gorge queues the granted keyword's enters-the-battlefield trigger
// from scriptfacts.GrantedKeywordTrigger, the printed kwLivingWeapon
// expansion's shape with its body inline (the remembered Germ token, then the
// source attached to it), so the grant lowers like the printed line's trigger:
// a latent ability the static lists among its Grant entries, one per granting
// static (each living weapon is its own trigger, CR 113.2c).

// splitLivingWeaponGrant takes every bare "Living Weapon" term out of a `&`
// keyword list: the rest of the list and how many there were.
func splitLivingWeaponGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Living Weapon" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// livingWeaponGrantTrigger lowers the trigger a granted living weapon gives
// its holder as a latent ability and returns its index, one per card. The
// kernel lists it once per granting static among the holder's granted
// triggers (grantedAbilitiesN).
func (lc *lowerCtx) livingWeaponGrantTrigger() (int, error) {
	const name = "GrantedLivingWeapon"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	tp := scriptfacts.GrantedKeywordTrigger("Living Weapon")
	if tp == nil {
		return 0, refuse("granted Living Weapon lowering")
	}
	t := *tp
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Living Weapon lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}
