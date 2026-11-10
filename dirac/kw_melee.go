package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Melee granted by a static (CR 702.121a; Glory to the Empire!'s "creatures
// you control have melee"): kgen lowers the grant as a latent Attacks Self
// trigger marked kop.Ability.MeleeGrant, whose pump is +1/+1 (the one
// opponent a two-player declaration attacks). Each instance triggers
// separately (CR 702.121b): grantedAbilitiesN lists one per granting static.
// A printed Melee (Ferocious Colonist) is an ordinary printed Attacks Self
// trigger beside them, absent while the face is down, so a grant on top of
// it is its own instance and on a face-down holder the only one: gorge
// discounts the printed line only while it exists.
//
// gorge queues a granted Melee in its own keyword walk (rules/melee.go
// checkGrantedAttackPumpTriggers), not among the triggers statics and effects
// grant by AddTrigger$ (rules/trigger_granted.go). Its place among them
// follows gorge's per-object walk (rules/trigger_match.go): a face whose
// printed lines may observe the declaration (kop.CardDef.AtkFace) is walked
// first, then the AddTrigger$ grants, then the keyword walks; a face-down
// permanent or a face without such a line takes the printed-free path, the
// keyword walks before the AddTrigger$ grants. slotTriggers lists the other
// grants without the Melee ones and these here, before or after them. Both
// paths run only on a table with a Melee grant (genPrep.meleeGrant).

// meleeProbe stands for a granted Melee trigger in slotTriggers' event
// test: an event that cannot fire an Attacks trigger skips the Melee walk.
var meleeProbe = kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttacks}}

// meleeGrantsFirst reports whether the permanent at sl takes gorge's
// printed-free path: its granted Melee triggers are listed before its other
// granted triggers.
func (s *State) meleeGrantsFirst(k *prep, sl Slot) bool {
	return s.faceDown(k, sl) || !k.t.Cards[s.cardAt(sl)].AtkFace
}

// meleeGrantTriggers calls f for the granted Melee triggers of the
// permanent at sl that ev accepts, in the grant walk's order.
func (s *State) meleeGrantTriggers(k *prep, sl Slot, ev func(a *kop.Ability) bool, f func(l trigListener, a *kop.Ability)) {
	p := &s.perms[sl]
	s.grantedAbilitiesN(k, sl, func(c CardID, ab uint8, n int) {
		a := &k.t.Cards[c].Abilities[ab]
		if a.MeleeGrant && a.Kind == kop.AbTriggered && ev(a) {
			for ; n > 0; n-- {
				f(trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: c, ab: ab, host: s.g.grantSrc}, a)
			}
		}
	})
}
