package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
			// ValidPlayer$ reads the TAPPER (gorge tapsMatches/TapActor), and a
			// mana tap is the payer's; the listener's controller is only the
			// reference for "you"/"opponent" (tapsWhoseOK).
			if !whoseOK(a.Trig.Whose, s.perms[w].ctrl, p) {
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
	if k.gen == nil || !k.gen.anyTapsDefer || !s.subs[kop.TrigTaps].any() {
		return false
	}
	_, d := s.tapsWatchClass(k, p, sl)
	return d
}

// scriptTapsDefer is p's activatable Taps-deferred mana sources, a slot
// mask: the census gap a live Defer-class watcher makes of a source's tap.
// Which sources defer is a runtime fact of the watchers alive, so no card
// flag reaches it (contrast gapMask's gfEachCol and friends); the walk is
// gapMask's, keyed on tapsDeferred. Only a tap is the source's mana
// activation for it -- gorge's tier classes planned taps
// (pay.PaymentPlanAbilityTier via paymentPlanSourceInterference) -- and the
// census gap the prefix search branches on (potentialPrefixScript) is a
// source whose missed ability is a plain one-mana-of-any-colour tap
// (Firdoch Core), whose relaxed alternatives (paymentPlanRelaxedAlternatives)
// are one mana per colour, or a plain one-mana tap of one fixed colour
// (Llanowar Elves under Cryoshatter): its one relaxed alternative, no colour
// ask (deferGapAnswers is 1). Left out, that source kept its census unit and
// the offer priced a plain cast with it, which gorge's census never lists and
// its autopay surface never scripts. Any other production stays unmodelled.
func (s *State) scriptTapsDefer(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.anyTapsDefer {
		return slotMask{}
	}
	var m slotMask
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		sl := b.first()
		if !s.planDeferred(k, p, sl) {
			continue
		}
		c := s.cardAt(sl)
		if s.manaGranted(k, sl) || k.gen.has(c, gfRestr|gfAnyR|gfEachCol|gfFilt|gfDeferred) {
			// Not the plain unrestricted any-colour shape the census gap is:
			// a restricted source's mana must float restricted (tapDeferFloat
			// adds plain pool mana), a granted one is grantCap/grantPrefix's,
			// an each-colour or filter one has its own arm, and a deferred one
			// is tapGap's gfDeferred arm. Each keeps the census unit the
			// availFor skip would otherwise drop (the skip is keyed on this
			// mask).
			continue
		}
		pc := s.manaPCGen(k, sl)
		if pc.manaAb < 0 || !pc.manaTap || pc.manaN != 1 || !tapsDeferColour(pc.manaCol) ||
			pc.manaChosen || pc.manaOne || pc.manaMulti || pc.manaDyn {
			continue
		}
		if int(pc.manaAb) >= len(k.t.Cards[c].Abilities) || k.t.Cards[c].Abilities[pc.manaAb].ManaRestrict != 0 {
			continue // the ability is restricted (Pillar of the Paruns): the gap floats plain mana
		}
		pm := &s.perms[sl]
		if pm.flags&pfTapped != 0 || pm.flags&pfSick != 0 && s.isCreature(k, sl) && s.kw(k, sl)&kop.KwHaste == 0 || s.genLost(k, sl) ||
			k.gen.anyCantAct && s.actForbidden(k, sl, true) { // a CantBeActivated lock (cast_cantactivate.go)
			continue
		}
		m = m.with(sl)
	}
	return m
}

// tapsDeferColour is the production scriptTapsDefer models: any colour, or
// exactly one of WUBRG.
func tapsDeferColour(c kop.ColorMask) bool {
	return c == kop.ColAny || c != 0 && c&^kop.ColAny == 0 && c&(c-1) == 0
}

// addTapsDeferUnits adds the census unit of the Taps-deferred gap at sl to a:
// its own mana and the extra a StatTapExtra carrier brings with the tap
// (Badgermole Cub; gorge's relaxed alternative carries it too,
// potential_plan.go potentialTapExtra). A source the extra cannot be priced on
// stalls (srcExtra).
func (s *State) addTapsDeferUnits(k *prep, sl Slot, a *avail) {
	pc := s.manaPCGen(k, sl)
	a.addUnits(s.srcMaskAt(pc, sl), srcYield(pc))
	if k.gen.anyTapExtra {
		xv := s.srcExtra(k, sl, pc)
		for c := range xv {
			if xv[c] > 0 {
				a.addUnits(1<<c, xv[c])
			}
		}
	}
}

// deferGapAnswers is how many colour answers a Taps-deferred gap's
// activation poses: one per colour its production names (gorge's search
// branches on the mana ask's options, manaChoiceColours W U B R G).
func (s *State) deferGapAnswers(k *prep, sl Slot) uint8 {
	return uint8(bits.OnesCount8(uint8(s.manaPCGen(k, sl).manaCol)))
}

// tapDeferFloat floats the Taps-deferred source at sl's mana in the
// answer's colour: tapGap's Taps-defer arm, plain pool mana (tapGapChoice
// and the prefix player tap it first, so manaTapped has queued the
// watcher's trigger for flushPrefixTriggers to place before the play).
func (s *State) tapDeferFloat(k *prep, p uint8, sl Slot, c int) {
	pc := s.manaPCGen(k, sl)
	mask := uint8(pc.manaCol)
	for i := 0; i < c; i++ {
		mask &= mask - 1
	}
	col := bits.TrailingZeros8(mask)
	s.pl[p].pool[col]++
	s.addExtra(k, p, sl, pc) // the tap's StatTapExtra mana floats beside it (addTapsDeferUnits)
	s.artTag(k, p, sl, col)
	if k.painMana && pc.painCol != 0 && pc.painOnly {
		s.painDamage(k, sl, p, int(pc.painN)) // a pain-only source's rider resolves with its mana (Elves of Deep Shadow)
	}
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
