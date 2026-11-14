package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// xCountActivatedOK reports whether a's "X target <objects>" slot (TargetMin$ X
// and TargetMax$ X, a TgfBoundsV slot whose bounds are both the announced X)
// may lower on an activated ability: a printed battlefield ability whose cost
// carries a mana {X} (Orcish Settlers' {X}{X}{R}, {T}, sacrifice: destroy X
// target lands). gorge announces the X before the targets (CR 602.2b, 601.2b-c)
// and offers only an X its target ask can answer (rules/cast_subcounter.go
// xAsk, targetsAnnounceableAt reads the ability's own SA); the kernel's X ask
// filters the same way for an ability entry (addXOptions, xAnswerable), and
// X = 0 needs no target, so the offer is never short. A trigger has no
// announced X, and a granted, hand or graveyard ability has no source
// permanent for the probe, so those stay refused.
func xCountActivatedOK(a *lAbility, ka *kop.Ability) bool {
	return ka.Kind == kop.AbActivated && !a.Latent && (a.Zone == 0 || a.Zone == zBattlefield) &&
		a.Trigger == nil && ka.Cost.Mana.X > 0
}
