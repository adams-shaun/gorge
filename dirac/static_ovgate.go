package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// GrantedStaticStats counts override-granted static hits.
var GrantedStaticStats struct{ Hits atomic.Int64 }

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
	// a copy exception's grant (CopyPermanent AddStaticAbilities$) is a
	// copiable value (CR 707.9b): layer 1, so a loss of all abilities (layer 6)
	// ends it, whatever the timestamps
	for i := 0; i < int(s.g.nfx) && !hit && !lost; i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc && k.t.Overrides[r.r.idx-1].GrantStatics&g != 0 {
			hit = true
		}
	}
	return hit
}

// ovGateReach reports whether a live override record could switch on an
// OvGate-gated static: the combat guards' cheap companion to statM. The static
// lives on the granting card's face, not on the holder's permanent, and statM
// holds only battlefield permanents whose OWN card has a static, so a grant
// whose source has left the battlefield is reachable only through the record.
func (s *State) ovGateReach(k *prep) bool {
	return k.gen != nil && k.gen.anyOvGateStat && s.fxM.any()
}

// grantedStaticFlag reads the statics of kind sk that a live override record
// on sl grants: an Animate staticAbilities$ body lowered with an OvGate bit
// (kgen animateGrantedStatic; Streetwise Negotiator's Backup rider, CR
// 702.165) whose record on the holder carries the matching
// kop.Override.GrantStatics bit. gorge registerAnimateStaticAbilities
// registers the granted static scoped to the animated object (CR 611.3a: an
// ability an effect grants is the object's own static), so its filter and
// condition resolve against the HOLDER, and a record older than sl's loss of
// all abilities grants nothing (CR 613.1f, 613.7; ovGranted's rule). hit
// stays false when no record names such a static.
func (s *State) grantedStaticFlag(k *prep, sk kop.StaticKind, sl Slot, hit *bool) {
	if *hit {
		return
	}
	lts, lost := uint16(0), false
	known := false
	pm := &s.perms[sl]
	s.eachOvRec(sl, func(r ovRec) {
		if *hit || r.idx == 0 {
			return
		}
		o := &k.t.Overrides[r.idx-1]
		if o.GrantStatics == 0 {
			return
		}
		if !known {
			lts, lost = s.lossAt(k, sl)
			known = true
		}
		if lost && r.ts < lts {
			return
		}
		cd := &k.t.Cards[r.src]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic {
				continue
			}
			st := &a.Static
			if st.OvGate == 0 || o.GrantStatics&st.OvGate == 0 || st.Kind != sk {
				continue
			}
			ctx := evalCtx{src: pm.obj, srcSl: sl, ctrl: pm.ctrl}
			if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) {
				continue
			}
			if !s.matchPermGen(k, st.Filter, sl, pm.ctrl, sl, layerFull) {
				continue
			}
			GrantedStaticStats.Hits.Add(1)
			*hit = true
			return
		}
	})
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

// grantedCantBlockBy reports whether a live override record on attacker a
// grants a CantBlockBy static whose filters match a and blocker b: an Animate
// staticAbilities$ body with an OvGate bit (Chomping Kavu's Backup rider,
// "can't be blocked by creatures with power 2 or less"). The static lives on
// the card that made the record, so the ordinary source-static walk
// (blockForbiddenGen's eachStatic) never reaches the holder; this is its
// gated arm, with the same filter/condition reads grantedStaticFlag uses.
func (s *State) grantedCantBlockBy(k *prep, a, b Slot) bool {
	lts, lost := uint16(0), false
	known := false
	hit := false
	pm := &s.perms[a]
	s.eachOvRec(a, func(r ovRec) {
		if hit || r.idx == 0 {
			return
		}
		o := &k.t.Overrides[r.idx-1]
		if o.GrantStatics == 0 {
			return
		}
		if !known {
			lts, lost = s.lossAt(k, a)
			known = true
		}
		if lost && r.ts < lts {
			return
		}
		cd := &k.t.Cards[r.src]
		for i := range cd.Abilities {
			ab := &cd.Abilities[i]
			if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatCantBlockBy {
				continue
			}
			st := &ab.Static
			if st.OvGate == 0 || o.GrantStatics&st.OvGate == 0 {
				continue
			}
			ctx := evalCtx{src: pm.obj, srcSl: a, ctrl: pm.ctrl}
			if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) {
				continue
			}
			if !s.matchPermGen(k, st.Filter, a, pm.ctrl, a, layerFull) {
				continue
			}
			if st.Filter2 != 0 && !s.matchPermGen(k, st.Filter2, b, pm.ctrl, a, layerFull) {
				continue
			}
			if k.anyPwVsSrc && !s.pwVsSrc(k, b, a, st.PwVsSrc) {
				continue
			}
			hit = true
			return
		}
	})
	return hit
}
