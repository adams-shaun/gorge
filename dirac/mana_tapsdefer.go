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
			if a.Trig.Flags5&kop.Tf5TapperNonActive != 0 && p == s.active {
				continue // Activator$ Player.NonActive: the payer taps on their own turn
			}
			if a.Trig.Flags&kop.TfSelf != 0 {
				// the source's own Pain-class line (City of Brass) is priced
				// with its printed mana ability (prepSelfTapsPain), and a
				// self line classes no other source
				continue
			}
			if !s.matchPermGen(k, a.Trig.Filter, sl, s.perms[w].ctrl, w, layerFull) {
				continue
			}
			// The gates gorge's real matcher applies (triggers_gen.go: FirstTime$
			// reads mechTappedTurn, YourTurn$ the turn's active player): a spent
			// or wrong-turn watcher matches no tap, so the source is an
			// ordinary planner unit (Shuri's crewed Quinjet beside a spent
			// Captain America).
			if a.Trig.Flags2&kop.Tf2FirstTime != 0 && s.mSl(k, mechTappedTurn, sl) {
				continue // an earlier tap this turn: FirstTime$ is false
			}
			if a.Trig.Flags2&kop.Tf2YourTurn != 0 && s.active != s.perms[w].ctrl {
				continue // not the watcher's controller's turn
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
	if k.gen == nil || !(k.gen.anyTapsDefer || k.gen.anyManaRider) {
		return slotMask{}
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if a, ok := s.riderAbilityOf(k, sl); ok {
			// A bounded rider source: the planner defers its first ability
			// (riderDeferred) or, with a plain first ability, its later
			// rider (Grove of the Burnwillows). Both are prefix gaps; the
			// activation pays the ability's own cost (mana_rider.go).
			if s.manaGranted(k, sl) || k.gen.has(c, gfRestr|gfAnyR|gfEachCol|gfFilt|gfDeferred) ||
				!s.riderActivatable(k, p, sl, a) {
				continue
			}
			m = m.with(sl)
			continue
		}
		if !s.planDeferred(k, p, sl) {
			continue
		}
		if s.splitDeferred(k, sl) {
			if s.splitUsable(k, sl) {
				m = m.with(sl) // a Condition-split source (mana_condsplit.go): its relaxed unit and answers are the live gate's
			}
			continue
		}
		if !s.manaGranted(k, sl) && s.restrTapsGap(k, p, sl) {
			// A restricted source whose single restricted ability one fixed
			// colour makes (Hydraulic Helper): the same gap, its mana floating
			// restricted (tapDeferFloat's restrTaps arm, floatRestrAb), when
			// the source can be activated at all.
			if s.tapsGapActivatable(k, sl) {
				m = m.with(sl)
			}
			continue
		}
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
		if !s.tapsGapActivatable(k, sl) {
			continue
		}
		m = m.with(sl)
	}
	return m
}

// tapsGapActivatable: the Taps-deferred gap at sl can be tapped for mana now:
// untapped, not summoning-sick, still its lowered card, and under no
// CantBeActivated lock (cast_cantactivate.go). Every script gap arm that taps
// its source asks it.
func (s *State) tapsGapActivatable(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfTapped == 0 && !s.activationSick(k, sl) && !s.genLost(k, sl) &&
		!(k.gen.anyCantAct && s.actForbidden(k, sl, true))
}

// tapsDeferColour is the production scriptTapsDefer models: any colour, or
// exactly one of WUBRG.
func tapsDeferColour(c kop.ColorMask) bool {
	return c == kop.ColAny || c != 0 && c&^kop.ColAny == 0 && c&(c-1) == 0
}

// restrTapsGapShape: the source at sl is a restricted planner source whose
// only mana production is one restricted ability of one fixed colour
// (Hydraulic Helper's artifact-spells {U}): the Taps-deferred gap the script
// route funds, floating its mana restricted (floatRestrAb). A source with a
// choice between restricted abilities (Steelswarm Operator) is no such gap.
func (s *State) restrTapsGapShape(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	if k.gen == nil || k.gen.rs == nil || k.cards[c].manaAb >= 0 || !k.gen.has(c, gfRestrSrc) {
		return false
	}
	n := 0
	var rec *restrRec
	for i := range k.gen.rs.recs {
		if k.gen.rs.recs[i].card == c {
			n++
			rec = &k.gen.rs.recs[i]
		}
	}
	if n != 1 || rec.kind != rkExt {
		return false
	}
	a := &k.t.Cards[c].Abilities[rec.ab]
	return a.Kind == kop.AbMana && !a.ManaEach &&
		a.AFlags&(kop.AfManaChoice|kop.AfManaDynamic|kop.AfManaChosen) == 0 &&
		oneColour(a.ManaColors&kop.ColAny) &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// restrTapsGap: the source at sl is Taps-deferred (a live Defer-class watcher
// with the gates gorge's matcher applies, tapsWatchClass) and of the script
// route's restricted shape.
func (s *State) restrTapsGap(k *prep, p uint8, sl Slot) bool {
	return s.tapsDeferred(k, p, sl) && s.restrTapsGapShape(k, sl)
}

// addTapsDeferUnits adds the census unit of the Taps-deferred gap at sl to a:
// its own mana and the extra a StatTapExtra carrier brings with the tap
// (Badgermole Cub; gorge's relaxed alternative carries it too,
// potential_plan.go potentialTapExtra). A restricted gap's unit is its
// restricted ability's production, priced beside the restriction as any gap's
// is (the play pays it from the floating batches). A source the extra cannot
// be priced on stalls (srcExtra).
func (s *State) addTapsDeferUnits(k *prep, sl Slot, av *avail) {
	if s.restrTapsGapShape(k, sl) {
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
		av.addUnits(uint8(ab.ManaColors&kop.ColAny), max(ab.ManaN, 1))
		return
	}
	if a, ok := s.riderAbilityOf(k, sl); ok {
		n := max(a.ManaN, 1)
		if a.ManaEach {
			for col := 0; col < kop.NumManaKinds; col++ {
				if uint8(a.ManaColors)&(1<<uint(col)) != 0 {
					av.addUnits(1<<uint(col), n)
				}
			}
			return
		}
		av.addUnits(uint8(a.ManaColors), n)
		return
	}
	pc := s.manaPCGen(k, sl)
	if s.splitDeferred(k, sl) {
		pc = s.splitCensusPC(k, sl) // the root's production plus the arm's while its gate holds (mana_condsplit.go)
	}
	av.addUnits(s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc))
	if k.gen.anyTapExtra {
		xv := s.srcExtra(k, sl, pc)
		for c := range xv {
			if xv[c] > 0 {
				av.addUnits(1<<c, xv[c])
			}
		}
	}
}

// deferGapAnswers is how many colour answers a Taps-deferred gap's
// activation poses: one per colour its production names (gorge's search
// branches on the mana ask's options, manaChoiceColours W U B R G), plus
// the plain first ability as answer 0 when the rider is a later ability
// (Grove of the Burnwillows; riderPlainFirst).
func (s *State) deferGapAnswers(k *prep, sl Slot) uint8 {
	if s.restrTapsGapShape(k, sl) {
		return 1 // one fixed colour: no colour ask
	}
	if a, ok := s.riderAbilityOf(k, sl); ok {
		if a.ManaEach {
			return 1 // every colour at once: no colour ask
		}
		n := uint8(bits.OnesCount8(uint8(a.ManaColors)))
		if s.riderPlainFirst(k, sl) {
			n++
		}
		return n
	}
	return uint8(bits.OnesCount8(uint8(s.manaPCGen(k, sl).manaCol)))
}

// tapDeferPlain floats a rider source's plain first ability at sl, the
// answer-0 activation of a gap whose rider comes later (riderPlainFirst):
// one fixed colour into the plain pool, exactly tapDeferFloat's non-rider
// arm.
func (s *State) tapDeferPlain(k *prep, p uint8, sl Slot) {
	pc := s.manaPCGen(k, sl)
	col := bits.TrailingZeros8(uint8(pc.manaCol))
	s.pl[p].pool[col]++
	s.addExtra(k, p, sl, pc) // the tap's StatTapExtra mana floats beside it (addTapsDeferUnits)
	s.artTag(k, p, sl, col)
	if k.painMana && pc.painCol != 0 && pc.painOnly {
		s.painDamage(k, sl, p, int(pc.painN))
	}
}

// tapDeferFloat floats the Taps-deferred source at sl's mana in the
// answer's colour: tapGap's Taps-defer arm, plain pool mana (tapGapChoice
// and the prefix player tap it first, so manaTapped has queued the
// watcher's trigger for flushPrefixTriggers to place before the play).
func (s *State) tapDeferFloat(k *prep, p uint8, sl Slot, c int) {
	if a, ok := s.riderAbilityOf(k, sl); ok {
		s.manaRiderFloat(k, p, sl, a, c)
		return
	}
	if s.restrTapsGapShape(k, sl) {
		// the restricted gap's mana floats restricted (tapGap's restr arm):
		// the play pays it from the floating batches the payment admits
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
		if !s.floatRestrAb(k, p, sl, ab, bits.TrailingZeros8(uint8(ab.ManaColors)), max(ab.ManaN, 1)) {
			s.capFull(StallRestrSrc) // every batch is taken: never a silent spill
		}
		return
	}
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

// StallRestrDefer: a non-spell payment only a Taps-deferred restricted mana
// source can fund (restrSrcAffords). gorge pays it through its prefix search,
// tapping the source first and choosing among its restricted abilities; the
// kernel's Taps-deferred gap route models one plain production.
const StallRestrDefer = 160

func init() { stallNames[StallRestrDefer] = "restrdefer" }

// selfTapsPainN is the damage of card c's own Pain-class "whenever CARDNAME
// becomes tapped" line (kgen admits one beside a printed mana ability only as
// the vetted "deals N damage to you" body and a plain {T} ability: City of
// Brass), and whether it has one.
func selfTapsPainN(c *kop.CardDef) (uint8, bool) {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags&kop.TfSelf != 0 &&
			a.Trig.TapPlan == kop.TapPlanPain && a.AFlags&kop.AfLatent == 0 &&
			(a.Trig.Zone == kop.ZoneNone || a.Trig.Zone == kop.ZoneBattlefield) {
			return a.Trig.TapPainN, true
		}
	}
	return 0, false
}

// prepSelfTapsPain prices a card with a self Pain-class Taps line as a
// pain-only mana source (mana_pain.go): gorge's planner reads the trigger as
// the source's own consequence (pay.PaymentPlanSelfDamageTrigger), a
// last-resort unit priced by its damage, exactly a pain-only source's tier.
// The damage itself is the trigger's, queued by the tap (manaTapped) and
// resolved from the stack, so painDamage deals none for it.
func prepSelfTapsPain(c *kop.CardDef, pc *pcard) bool {
	if pc.manaAb < 0 || pc.painCol != 0 {
		return false
	}
	n, ok := selfTapsPainN(c)
	a := &c.Abilities[pc.manaAb]
	if !ok || !primaryTapAb(a) {
		return false
	}
	pc.painCol, pc.painN, pc.painOnly = a.ManaColors, n, true
	return true
}
