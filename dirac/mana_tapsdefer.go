package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Taps watchers and the payment plan. gorge classifies every source a planned
// tap would touch (pay.PaymentPlanAbilityTier): a source whose {T} fires a
// "whenever a creature you control becomes tapped" watcher with an unvetted
// body (Judge of Currents' gain life, Magda's Treasure, Quest for Renewal's
// counter) is TierDeferred -- the census never lists it, so autopay never
// taps it (manual only) -- and one whose watcher deals vetted literal damage
// to the payer is a last-resort unit priced by that damage (Pain). Vetted
// damage that cannot reach the payer leaves the source an ordinary unit.
// kgen stamps the class on the watcher (kop.Trigger.TapPlan, closed world:
// lower_tapsclass.go); this file reads it per source.

// DebugNoTapsDefer drops the classification (every source an ordinary unit).
// The lockstep's mutation test flips it to prove the pool exercises it.
var DebugNoTapsDefer bool

// tapsWatchClass classifies a tap of sl for mana against the battlefield's
// lowered Taps watchers: painN is the damage the tap deals its payer (the
// Pain-class watchers' amounts, summed), deferSrc whether a Defer-class
// watcher matches. p is the payer. Tables without the family walk nothing.
func (s *State) tapsWatchClass(k *prep, p uint8, sl Slot) (painN int, deferSrc bool) {
	if k.gen == nil || !(k.gen.anyTapsDefer || k.gen.anyTapsPain) || DebugNoTapsDefer {
		return 0, false
	}
	for m := s.subs[kop.TrigTaps]; m.any(); m = m.rest() {
		w := m.first()
		if !s.occ.has(w) || k.gen.flags[s.cardAt(w)]&(gfTapsDefer|gfTapsPain) == 0 || s.genLost(k, w) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(w)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigTaps || a.AFlags&kop.AfLatent != 0 ||
				a.Trig.TapPlan != kop.TapPlanDefer && a.Trig.TapPlan != kop.TapPlanPain ||
				a.Trig.Zone != kop.ZoneNone && a.Trig.Zone != kop.ZoneBattlefield {
				continue
			}
			if !s.matchPermGen(k, a.Trig.Filter, sl, s.perms[w].ctrl, w, layerFull) {
				continue
			}
			if a.Trig.TapPlan == kop.TapPlanDefer {
				deferSrc = true
			} else {
				painN += int(a.Trig.TapPainN)
			}
		}
	}
	if painN > 255 {
		painN = 255
	}
	return painN, deferSrc
}

// tapsDeferred reports whether sl is a source gorge's planner never taps
// (tapsWatchClass's Defer class).
func (s *State) tapsDeferred(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.anyTapsDefer {
		return false
	}
	_, d := s.tapsWatchClass(k, p, sl)
	return d
}

// planTiered is plan with gorge's tiers: the sources a Taps watcher
// defers are no unit of the first pass. gorge's planner never lists them, but
// a play the planner then proves insufficient is paid through the potential
// play's prefix (they are census gaps, activated first), which taps them: so
// when no plan exists without them a second pass plans with them, in
// battlefield order like any source.
func (s *State) planTiered(k *prep, p uint8, pk *planner, exclude Obj) {
	if k.gen == nil || !k.gen.anyTapsDefer || DebugNoTapsDefer {
		s.plan(k, p, pk, exclude)
		return
	}
	saved := *pk
	pk.try = true
	s.plan(k, p, pk, exclude)
	if pk.found {
		pk.try = saved.try
		return
	}
	*pk = saved
	pk.tapsIn = true
	s.plan(k, p, pk, exclude)
}
