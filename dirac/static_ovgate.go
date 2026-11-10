package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugNogNoGrant withholds every override-granted card static (the Nog,
// Ambitious Cadet mutation test): the graveyard land permission and the
// extra land play an Animate staticAbilities$ grant switches on.
var DebugNogNoGrant bool

// ovGranted reports whether permanent sl carries a live override record
// whose Override.GrantStatics has a bit of g: the permanent's own static with
// that kop.Static.OvGate is in force (gorge registerAnimateStaticAbilities
// registers each granted body as a continuous effect scoped to the animated
// object, riding the animation's lifetime). A record older than the
// permanent's loss of all abilities grants nothing (CR 613.6, 613.7).
func (s *State) ovGranted(k *prep, sl Slot, g uint8) bool {
	if DebugNogNoGrant || sl == NoSlot || !s.fxM.has(sl) {
		return false
	}
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachOvRec(sl, func(r ovRec) {
		if k.t.Overrides[r.idx-1].GrantStatics&g != 0 && (!lost || r.ts >= lts) {
			hit = true
		}
	})
	return hit
}

// ovGateKeptOnLoss: a's effect outlives its source's loss of all abilities
// because the override record granting it is live and not older than the
// loss (CR 613.1f, 613.7: The Irencrag's "Equipped creature gets +3/+3"
// comes from the same Animate that makes it lose all other abilities).
func (s *State) ovGateKeptOnLoss(k *prep, sl Slot, a *kop.Ability) bool {
	return a.Kind == kop.AbStatic && a.Static.OvGate != 0 && s.ovGranted(k, sl, a.Static.OvGate)
}

// keptOnLossAt is keptOnLoss for permanent sl's ability a, with the
// override-granted exception (ovGateKeptOnLoss).
func (s *State) keptOnLossAt(k *prep, sl Slot, a *kop.Ability) bool {
	return keptOnLoss(k.t, a) || s.ovGateKeptOnLoss(k, sl, a)
}
