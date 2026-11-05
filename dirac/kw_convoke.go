package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Convoke (CR 702.51): the cast may tap untapped creatures for its mana
// cost. The announcement (gorge convokeAsk) runs after the tap costs and
// before X; each pick pays one owed colour pip or one generic. The greedy
// offer credit is gorge convokeCost; the actual picks are subtracted in
// pick order at payment, and the picked creatures cannot also be mana
// sources in the window.

const convokePickMask pickKind = 0x4400

// Mutation-test switches (dirac/oracle TestConvokeMutation): the lockstep
// must diverge when either is set.
var (
	// DebugConvokeTapAtPick taps a creature as it is picked, instead of at
	// payment (gorge EmitConvokeTaps at commit).
	DebugConvokeTapAtPick bool
	// DebugConvokeNoAbsorb drops the oracle's convokeAbsorbs predicate, so
	// its announcement offers picks gorge's Submit would reject.
	DebugConvokeNoAbsorb bool
)

// convokeDebugTapAtPick is the tap-at-pick mutation (cold, test only).
func (s *State) convokeDebugTapAtPick(o Obj) {
	if !DebugConvokeTapAtPick {
		return
	}
	sl := s.objs[o].slot
	if sl != NoSlot {
		s.perms[sl].flags |= pfTapped
	}
}

// convokeGenMode is the generic pick's mode: 5 for convoke, 6 for improvise
// (gorge's improvise_generic). Both subtract one generic.
func convokeGenMode(imp bool) uint8 {
	if imp {
		return 6
	}
	return 5
}

// tapPayCand reports whether the untapped permanent at sl may be announced:
// a convoke creature (not bestowed or reconfigured) or, for improvise, any
// artifact (gorge ConvokeAsk: Engine.IsArtifact, creatures included).
func (s *State) tapPayCand(k *prep, sl Slot, imp bool) bool {
	pp := &s.perms[sl]
	if pp.flags&pfTapped != 0 {
		return false
	}
	if imp {
		return s.typesAt(k, sl)&kop.TArtifact != 0
	}
	return pp.attached == NoSlot && s.isCreature(k, sl)
}

func isConvokePick(f pickKind) bool { return f&convokePickMask == convokePickMask }

// convokeCredit returns cost less the greedy convoke credit: in p's
// battlefield order, each untapped creature that is not bestowed or
// reconfigured pays the first owed colour among W,U,B,R,G it has, else one
// generic. Hybrid and {C} pips are never paid (gorge convokeCost).
func (s *State) convokeCredit(k *prep, p uint8, c CardID, o Obj, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyConvoke {
		return cost
	}
	fromHand := s.objs[o].zone == kop.ZoneHand // the offer: the card is still in the hand
	if !s.tapPays(k, p, c, o, fromHand) {
		return cost
	}
	imp := s.isImprovise(k, p, c, o, fromHand)
	var rem [5]uint8
	for i := range rem {
		rem[i] = cost.Pips[i]
	}
	gen := cost.Generic
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p&1, s.ctrlM[p&1].and(s.occ), ord[:], 0)
	for _, sl := range ord[:n] {
		if !s.tapPayCand(k, sl, imp) {
			continue
		}
		col := s.colorsAt(k, sl)
		paid := false
		for ci := 0; ci < 5 && !imp; ci++ {
			if rem[ci] > 0 && col&(1<<ci) != 0 {
				rem[ci]--
				paid = true
				break
			}
		}
		if !paid && gen > 0 {
			gen--
		}
	}
	for i := range rem {
		cost.Pips[i] = rem[i]
	}
	cost.Generic = gen
	return cost
}

// convokeApplyPicks subtracts the announced convoke picks from cost in pick
// order (gorge applyConvoke).
func (s *State) convokeApplyPicks(k *prep, cost kop.ManaCost) kop.ManaCost {
	for i := 0; i < int(s.bld().pickN); i++ {
		if !isConvokePick(s.bld().pickKs[i]) {
			continue
		}
		col := int(s.bld().pickKs[i] & 7)
		if col == 7 {
			// harmonize: the creature's power off the generic part (CR 702.180a)
			cost.Generic -= s.harmonizeReduce(k, s.objs[s.bld().picks[i]].slot, cost.Generic)
		} else if col < 5 {
			if cost.Pips[col] > 0 {
				cost.Pips[col]--
			}
		} else if cost.Generic > 0 {
			cost.Generic--
		}
	}
	return cost
}

// convokePickN counts the announced convoke picks.
func (s *State) convokePickN() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if isConvokePick(s.bld().pickKs[i]) {
			n++
		}
	}
	return n
}

func (s *State) pickedKind(o Obj, kind pickKind) bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && s.bld().pickKs[i] == kind {
			return true
		}
	}
	return false
}

func (s *State) convokePicked(o Obj) bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && isConvokePick(s.bld().pickKs[i]) {
			return true
		}
	}
	return false
}

func (s *State) convokePickCount() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if isConvokePick(s.bld().pickKs[i]) {
			n++
		}
	}
	return n
}

// convokeOptions poses the next convoke pick: one option per owed colour a
// creature has, then generic, in battlefield order; Done closes the
// announcement. A menu with no pick is not posed.
func (s *State) convokeOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if k.gen == nil || !k.gen.anyConvoke || e.kind != seSpell || !s.tapPaysEntry(k, e) {
		return false
	}
	if harmonizeEntry(&k.t.Cards[e.card], e) {
		return s.harmonizeOptions(k, d) // kw_harmonize.go
	}
	if int(s.bld().pickN) >= len(s.bld().picks) {
		s.capFull(StallConvoke)
		return false
	}
	// gorge poses the ask while its own (uncredited) budget still has a
	// slot an untapped creature could fill (pay.ConvokeAsk reads
	// manaToPay); the oracle then drops every pick the announced cost no
	// longer absorbs, so a fully paid announcement still poses a Done-only
	// menu. Offer only the picks the post-pick remainder absorbs.
	raw := s.buildCost(k)
	imp := s.isImprovise(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl))
	// gorge's ask closes at Max = min(#options, slots), the uncredited
	// Colored.Total()+Generic (pay.ConvokeAsk): an announcement whose picks
	// reach the slot count commits without a Done.
	slots := int(raw.Generic)
	for _, p := range raw.Pips {
		slots += int(p)
	}
	// gorge's ConvokeAsk Max is min(offered options, outstanding slots) and
	// the answer commits as soon as Max picks are made (the oracle's Apply),
	// with no Done-only re-pose. Count the first ask's options over the
	// unreduced cost and every eligible creature.
	var ord0 [MaxPerm]Slot
	n0 := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord0[:], 0)
	offered := 0
	for _, sl := range ord0[:n0] {
		pp := &s.perms[sl]
		if !s.tapPayCand(k, sl, imp) {
			continue
		}
		o := pp.obj
		if s.pickedKind(o, pkTapX) {
			continue
		}
		col := s.colorsAt(k, sl)
		for ci := 0; ci < 5 && !imp; ci++ {
			if raw.Pips[ci] != 0 && col&(1<<ci) != 0 {
				offered++
			}
		}
		if raw.Generic > 0 {
			offered++
		}
	}
	maxPicks := slots
	if offered < maxPicks {
		maxPicks = offered
	}
	if s.convokePickCount() >= maxPicks {
		return false
	}
	cost := s.convokeApplyPicks(k, raw)
	var rem [5]uint8
	for i := range rem {
		rem[i] = cost.Pips[i]
	}
	gen := cost.Generic
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	d.N = 0
	d.Kind, d.Sub, d.Player = DkChoose, ChGenConvoke, e.ctrl
	pose := false
	for _, sl := range ord[:n] {
		pp := &s.perms[sl]
		if !s.tapPayCand(k, sl, imp) {
			continue
		}
		o := pp.obj
		if s.convokePicked(o) || s.pickedKind(o, pkTapX) {
			continue
		}
		col := s.colorsAt(k, sl)
		for ci := 0; ci < 5 && !imp; ci++ {
			if raw.Pips[ci] == 0 || col&(1<<ci) == 0 {
				continue
			}
			pose = true
			if rem[ci] > 0 {
				d.add(MakeAction(AkChoose, uint8(o), 0, uint8(ci)))
				if d.N >= MaxOptions-1 {
					s.capFull(StallConvoke)
					break
				}
			}
		}
		if raw.Generic > 0 {
			pose = true
			if gen > 0 {
				if d.N >= MaxOptions-1 {
					s.capFull(StallConvoke)
					break
				}
				d.add(MakeAction(AkChoose, uint8(o), 0, convokeGenMode(imp)))
			}
		}
		if d.N >= MaxOptions-1 {
			s.capFull(StallConvoke)
			break
		}
	}
	if !pose {
		// Gorge's announcement is a single multi-pick decision: after a
		// pick it re-poses with only Done left (the oracle's repose), even
		// when no further creature can contribute. Only a first ask with
		// no option at all is skipped (pay.ConvokeAsk's len(Options)==0).
		if s.convokePickCount() == 0 {
			return false
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
	return true
}

// convokeHidden is the slots of the announced creatures (payConvoke and
// manaStep withhold them from the mana planner).
func (s *State) convokeHidden() slotMask {
	var hidden slotMask
	for i := 0; i < int(s.bld().pickN); i++ {
		if !isConvokePick(s.bld().pickKs[i]) {
			continue
		}
		if o := s.bld().picks[i]; s.objs[o].zone == kop.ZoneBattlefield {
			hidden = hidden.with(s.objs[o].slot)
		}
	}
	return hidden
}

// payConvoke pays the convoke cast's remaining cost from the pool, then taps
// the announced creatures in pick order (gorge EmitConvokeTaps). The picked
// creatures are hidden from the window as mana sources. If the window cannot
// pay, the cast is reversed (gorge abortCast) and false is returned.
func (s *State) payConvoke(k *prep, e *stackEntry, cost *kop.ManaCost, anyType, anyColor bool) bool {
	red := s.convokeApplyPicks(k, *cost)
	saved := s.fork()
	defer s.unfork()
	s.occ = s.occ.andNot(s.convokeHidden())
	ok, held := s.windowTapUseful(k, e.ctrl, &red, cost, !anyType && !anyColor, anyColor, s.bld().pipN == 0, false)
	if !ok {
		s.CopyFrom(saved)
		s.reverseCast(k)
		return false
	}
	if s.winPaused() {
		for c := range held {
			s.pl[e.ctrl].pool[c] += held[c]
		}
		s.occ = saved.occ
		return true // the pick is posed; the announced creatures tap when the window ends (mana_window.go)
	}
	s.payFromPoolRider(k, e.ctrl, &red, 0, anyColor)
	for c := range held {
		s.pl[e.ctrl].pool[c] += held[c]
	}
	s.occ = saved.occ
	tb := false
	for i := 0; i < int(s.bld().pickN); i++ {
		if !isConvokePick(s.bld().pickKs[i]) {
			continue
		}
		o := s.bld().picks[i]
		if s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		sl := s.objs[o].slot
		was := s.perms[sl].flags & pfTapped
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		if was == 0 && k.gen != nil && s.subs[kop.TrigTaps].any() {
			tb = tb || s.tapBatchOpen(k) // the announcement's taps are one tapping action (gorge EmitConvokeTaps; trig_tapall.go)
			s.fireTaps(k, sl)
		}
	}
	s.tapBatchClose(tb)
	return true
}

// convokeTargetAbort reports whether the pending convoke cast (or a Play's paid
// copy cast, copyCastPaid) can no longer
// be completed: the cost less the announced picks is unpayable from the pool
// and the CR 601.2g window (the announced creatures are withheld, as at
// payment). Gorge's affordableTargetCandidates probes the same window and
// withholds every candidate of such a cast, so a required target reverses it
// (cast_begin.go's "no legal target") and an up-to target asks nothing.
func (s *State) convokeTargetAbort(k *prep) bool {
	if k.gen == nil {
		return false
	}
	e := &s.bld().e
	if e.kind != seSpell || !s.tapPaysEntry(k, e) && !s.copyCastPaid(k, e) {
		return false
	}
	if e.flags&sfFree != 0 || s.bld().bfl&bflLowered != 0 && !s.copyCastPaid(k, e) {
		return false
	}
	cost := s.convokeApplyPicks(k, s.buildCost(k))
	saved := s.fork()
	defer s.unfork()
	hide := s.convokeHidden()
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkTapX && s.objs[s.bld().picks[i]].zone == kop.ZoneBattlefield {
			hide = hide.with(s.objs[s.bld().picks[i]].slot)
		}
	}
	s.occ = s.occ.andNot(hide)
	// gorge's affordableTargetCandidates prices the window from its census,
	// which leaves a tap-another source (Springleaf Drum) unpriced: the census
	// is incomplete, so the menu keeps its candidates however short the
	// window is (CR 601.2g; castWindowCensusComplete). A tap-another source
	// with a creature left to tap is such a source.
	if s.scriptTapX(k, e.ctrl).any() {
		s.CopyFrom(saved)
		return false
	}
	if k.gen.anyWindowGap {
		// a window-gap source (Bloom Tender) is one gorge's census cannot
		// price, so the menu keeps its candidates (the census-incomplete arm).
		for m := s.ctrlM[e.ctrl].and(s.occ); m.any(); m = m.rest() {
			if s.windowGap(k, e.ctrl, m.first()) {
				s.CopyFrom(saved)
				return false
			}
		}
	}
	ok, _ := s.windowTapRider(k, e.ctrl, &cost, false, s.bld().bfl&bflAnyColor != 0)
	s.CopyFrom(saved)
	return !ok
}

// ConvokeHarmonizeMode is the mode of a Harmonize announcement's option.
const ConvokeHarmonizeMode uint8 = 7

// FeatConvokeCands appends every permanent the announcement in progress offers
// at its first ask, picked ones included, in battlefield order, and reports
// whether it is an Improvise announcement.
func (s *State) FeatConvokeCands(out []Obj) ([]Obj, bool) {
	k := s.k()
	e := &s.bld().e
	raw := s.buildCost(k)
	imp := s.isImprovise(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl))
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	for _, sl := range ord[:n] {
		if !s.tapPayCand(k, sl, imp) || s.pickedKind(s.perms[sl].obj, pkTapX) {
			continue
		}
		off := raw.Generic > 0
		col := s.colorsAt(k, sl)
		for ci := 0; ci < 5 && !imp && !off; ci++ {
			off = raw.Pips[ci] != 0 && col&(1<<ci) != 0
		}
		if off {
			out = append(out, s.perms[sl].obj)
		}
	}
	return out, imp
}

// FeatConvokePicked reports whether o was already announced.
func (s *State) FeatConvokePicked(o Obj) bool { return s.convokePicked(o) }

// FeatConvokePickCount is the number of permanents announced so far.
func (s *State) FeatConvokePickCount() int { return s.convokePickCount() }

// FeatColorsOf is the battlefield permanent o's colours (bit i: W U B R G).
func (s *State) FeatColorsOf(o Obj) uint8 { return uint8(s.colorsAt(s.k(), s.objs[o].slot)) }
