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

// StallStackKwGrant: a spell carries a granted tap-paid keyword (Convoke or
// Improvise) on a Harmonize or alternative-cost route, whose announcement the
// kernel has not verified against gorge's, so the cast stalls rather than
// diverging. Unmodelled (kw_convoke.go convokeOptions). The mixed
// Convoke+Improvise and {X} shapes a grant reopens are modelled.
const StallStackKwGrant = 140

func init() { stallNames[StallStackKwGrant] = "stackkwgrant" }

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

// convokeCredit returns cost less the greedy tap-paid credit: convoke first,
// in p's battlefield order, each untapped creature that is not bestowed or
// reconfigured pays the first owed colour among W,U,B,R,G it has, else one
// generic; then improvise over the remaining generic, each untapped artifact
// not already committed by convoke pays one generic (gorge castOfferBase:
// convokeCost then improviseCost). Hybrid and {C} pips are never paid. A
// spell with both keywords is priced like gorge's offer too.
func (s *State) convokeCredit(k *prep, p uint8, c CardID, o Obj, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyConvoke {
		return cost
	}
	fromHand := s.objs[o].zone == kop.ZoneHand // the offer: the card is still in the hand
	convoke := s.hasConvoke(k, p, c, o, fromHand)
	improvise := s.isImprovise(k, p, c, o, fromHand)
	if !convoke && !improvise {
		return cost
	}
	var rem [5]uint8
	for i := range rem {
		rem[i] = cost.Pips[i]
	}
	gen := cost.Generic
	var used slotMask
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p&1, s.ctrlM[SeatIx(p)].and(s.occ), ord[:], 0)
	if convoke {
		for _, sl := range ord[:n] {
			if !s.tapPayCand(k, sl, false) {
				continue
			}
			col := s.colorsAt(k, sl)
			paid := false
			for ci := 0; ci < 5; ci++ {
				if rem[ci] > 0 && col&(1<<ci) != 0 {
					rem[ci]--
					paid = true
					break
				}
			}
			if !paid && gen > 0 {
				gen--
				paid = true
			}
			if paid {
				used = used.with(sl)
			}
		}
	}
	if improvise {
		for _, sl := range ord[:n] {
			if gen == 0 {
				break
			}
			if used.has(sl) || !s.tapPayCand(k, sl, true) {
				continue
			}
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

// convokeXCost folds x into cost and replays the announced picks in
// announcement order, reporting false when a pick would reduce nothing
// (gorge convokeAbsorbs at xAsk, xOpen=false).
func (s *State) convokeXCost(k *prep, cost kop.ManaCost, x int) (kop.ManaCost, bool) {
	cost = xFolded(cost, x)
	for i := 0; i < int(s.bld().pickN); i++ {
		if !isConvokePick(s.bld().pickKs[i]) {
			continue
		}
		col := int(s.bld().pickKs[i] & 7)
		switch {
		case col == 7:
			if cost.Generic == 0 {
				return cost, false // nothing for the tapped creature to reduce
			}
			cost.Generic -= s.harmonizeReduce(k, s.objs[s.bld().picks[i]].slot, cost.Generic)
		case col < 5:
			if cost.Pips[col] == 0 {
				return cost, false
			}
			cost.Pips[col]--
		default:
			if cost.Generic == 0 {
				return cost, false
			}
			cost.Generic--
		}
	}
	return cost, true
}

// convokeXCredit is the generic the announced picks can pay toward X (gorge
// xAsk's credit): a harmonize pick its creature's power, any other pick one.
func (s *State) convokeXCredit(k *prep) int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if !isConvokePick(s.bld().pickKs[i]) {
			continue
		}
		if s.bld().pickKs[i]&7 == 7 {
			if pw := s.power(k, s.objs[s.bld().picks[i]].slot); pw > 0 {
				n += pw
			}
			continue
		}
		n++
	}
	return n
}

// improvisePickCount counts the announced improvise (mode 6) picks.
func (s *State) improvisePickCount() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if isConvokePick(s.bld().pickKs[i]) && s.bld().pickKs[i]&7 == 6 {
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
	if cd := &k.t.Cards[e.card]; harmonizeEntry(cd, e) || cd.HasPlot || cd.HasSneak || cd.HasWarp || e.trigPl == trigPlWarpG ||
		cd.Mayhem || cd.HasWebSling || cd.HasMayFlash {
		// kgen refuses these routes beside a printed Convoke or Improvise, so
		// a live one is a grant. gorge's convokeAsk offers harmonize and
		// convoke/improvise picks together, and prices the alternative-cost
		// routes' announcement itself; the kernel's menu for them is
		// unverified under a grant, so stall rather than pose it. An impending
		// cast is not among them: buildCost prices its impending cost, the cost
		// gorge's ask reads (CR 702.176a, 702.51a).
		fh := s.castFromHandBy(e, e.ctrl)
		if s.hasConvoke(k, e.ctrl, e.card, e.obj, fh) || s.isImprovise(k, e.ctrl, e.card, e.obj, fh) {
			s.capFull(StallStackKwGrant)
			return false
		}
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
	fromHand := s.castFromHandBy(e, e.ctrl)
	convoke := s.hasConvoke(k, e.ctrl, e.card, e.obj, fromHand)
	improvise := s.isImprovise(k, e.ctrl, e.card, e.obj, fromHand)
	// A spell with both keywords (a grant beside a printed one) or an {X}
	// cost poses gorge's one mixed announcement: per keyword, the creature's
	// colour and generic picks, the artifact's improvise pick; with {X} the
	// generic picks are offered tentatively (the X ask prices them,
	// convokeXCost).
	hasX := raw.X > 0
	genOK := raw.Generic > 0 || hasX
	// gorge's ask closes at Max = min(#options, slots), the uncredited
	// Colored.Total()+Generic (pay.ConvokeAsk): an announcement whose picks
	// reach the slot count commits without a Done.
	// Only a Convoke creature's colour pick pays a coloured pip (CR 702.51a);
	// an Improvise artifact pays generic only (CR 702.126a), so gorge counts
	// the pips as slots only when the ask offers a colour pick.
	slots := int(raw.Generic)
	pipSlots := 0
	for _, p := range raw.Pips {
		pipSlots += int(p)
	}
	colourOpt := false
	// gorge's ConvokeAsk Max is min(offered options, outstanding slots) and
	// the answer commits as soon as Max picks are made (the oracle's Apply),
	// with no Done-only re-pose. Count the first ask's options over the
	// unreduced cost and every eligible creature.
	var ord0 [MaxPerm]Slot
	n0 := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord0[:], 0)
	offered := 0
	for _, sl := range ord0[:n0] {
		pp := &s.perms[sl]
		cc := convoke && s.tapPayCand(k, sl, false)
		ic := improvise && s.tapPayCand(k, sl, true)
		if !cc && !ic || s.pickedKind(pp.obj, pkTapX) {
			continue
		}
		col := s.colorsAt(k, sl)
		for ci := 0; ci < 5 && cc; ci++ {
			if raw.Pips[ci] != 0 && col&(1<<ci) != 0 {
				offered++
				colourOpt = true
			}
		}
		if cc && genOK {
			offered++
		}
		if ic && genOK {
			offered++
		}
	}
	if colourOpt {
		slots += pipSlots
	}
	maxPicks := slots
	if hasX || offered < maxPicks {
		maxPicks = offered
	}
	if s.convokePickCount() >= maxPicks {
		return false
	}
	// Every route filters by the post-pick remainder: gorge's Submit gate
	// (validateCastContributions) refuses a pick the outstanding cost cannot
	// absorb on a may-play or alternative-cost cast too (CR 702.51a), and the
	// oracle offers only the picks that gate admits.
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
		cc := convoke && s.tapPayCand(k, sl, false)
		ic := improvise && s.tapPayCand(k, sl, true)
		if !cc && !ic {
			continue
		}
		o := pp.obj
		if s.convokePicked(o) || s.pickedKind(o, pkTapX) {
			continue
		}
		col := s.colorsAt(k, sl)
		for ci := 0; ci < 5 && cc; ci++ {
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
		for g := 0; g < 2 && genOK; g++ {
			if g == 0 && !cc || g == 1 && !ic {
				continue
			}
			pose = true
			if gen > 0 || hasX {
				if d.N >= MaxOptions-1 {
					s.capFull(StallConvoke)
					break
				}
				d.add(MakeAction(AkChoose, uint8(o), 0, convokeGenMode(g == 1)))
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
	folded := xFolded(*cost, int(e.x)) // the picks are announced before X, and pay a generic of its total
	red := s.convokeApplyPicks(k, folded)
	saved := s.fork()
	defer s.unfork()
	s.occ = s.occ.andNot(s.convokeHidden())
	ok, held := s.windowTapUseful(k, e.ctrl, &red, &folded, !anyType && !anyColor, anyColor, s.bld().pipN == 0, false)
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
	s.payConvokeFromPool(k, e, &red, anyColor)
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
		if was == 0 {
			if k.gen != nil && s.subs[kop.TrigTaps].any() {
				tb = tb || s.tapBatchOpen(k) // the announcement's taps are one tapping action (gorge EmitConvokeTaps; trig_tapall.go)
				s.fireTaps(k, sl)
			}
			s.markTapped(k, sl) // only a tap that happens marks
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
	cost := s.convokeApplyPicks(k, xFolded(s.buildCost(k), int(e.x)))
	return s.targetWindowShort(k, e, &cost, true)
}

// targetWindowShort reports whether the cost of the cast being built cannot
// be met by the pool and the CR 601.2g window, the census gorge's
// affordableTargetCandidates runs before a target menu: when it is short
// every candidate is withheld. hidePicks withholds the announced convoke
// creatures from the window, as at payment.
func (s *State) targetWindowShort(k *prep, e *stackEntry, cost *kop.ManaCost, hidePicks bool) bool {
	if s.copyCastPaid(k, e) && s.copyCensusOpen(k, e) {
		// a paid copy cast's menu keeps its candidates on a census gap
		// (copyCensusOpen; affordableTargetCandidates' fallback), as the
		// copy abort helpers read it too.
		return false
	}
	saved := s.fork()
	defer s.unfork()
	hide := s.convokeHidden()
	for i := 0; hidePicks && i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkTapX && s.objs[s.bld().picks[i]].zone == kop.ZoneBattlefield {
			hide = hide.with(s.objs[s.bld().picks[i]].slot)
		}
	}
	s.occ = s.occ.andNot(hide)
	// gorge's affordableTargetCandidates prices the window from its census;
	// any source the census cannot price (a tap-another one with no creature
	// to tap, a restricted mana ability the payment admits but the probe does
	// not price, an each-colour source, a TapsForMana watcher that changes a
	// source's production) leaves it incomplete, and the menu keeps its
	// candidates however short the window is (CR 601.2c: targets are chosen,
	// and a cast only reverses at 601.2h when the payment cannot be met;
	// castWindowCensusComplete). This is the arm waterbendTargetAbort and
	// tapXTargetAbort share.
	if !s.windowCensusComplete(k, e) {
		s.CopyFrom(saved)
		return false
	}
	ok, _ := s.windowTapRider(k, e.ctrl, cost, false, s.bld().bfl&bflAnyColor != 0)
	s.CopyFrom(saved)
	return !ok
}

// ConvokeHarmonizeMode is the mode of a Harmonize announcement's option.
const ConvokeHarmonizeMode uint8 = 7

// FeatConvokeCands appends every permanent the announcement in progress offers
// at its first ask, picked ones included, in battlefield order (the union of
// the Convoke creatures and the Improvise artifacts), and reports whether the
// spell has Improvise. FeatConvokeKinds says which keywords are live.
func (s *State) FeatConvokeCands(out []Obj) ([]Obj, bool) {
	k := s.k()
	e := &s.bld().e
	raw := s.buildCost(k)
	convoke, improvise := s.FeatConvokeKinds()
	genOK := raw.Generic > 0 || raw.X > 0
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	for _, sl := range ord[:n] {
		if s.pickedKind(s.perms[sl].obj, pkTapX) {
			continue
		}
		cc := convoke && s.tapPayCand(k, sl, false)
		ic := improvise && s.tapPayCand(k, sl, true)
		off := genOK && (cc || ic)
		col := s.colorsAt(k, sl)
		for ci := 0; ci < 5 && cc && !off; ci++ {
			off = raw.Pips[ci] != 0 && col&(1<<ci) != 0
		}
		if off {
			out = append(out, s.perms[sl].obj)
		}
	}
	return out, improvise
}

// FeatConvokeKinds reports which tap-paid keywords the announcement in
// progress has: Convoke, Improvise (a spell may have both).
func (s *State) FeatConvokeKinds() (convoke, improvise bool) {
	k := s.k()
	e := &s.bld().e
	fh := s.castFromHandBy(e, e.ctrl)
	improvise = s.isImprovise(k, e.ctrl, e.card, e.obj, fh)
	// a spell without Improvise is a creature-tap announcement (Convoke, or
	// Harmonize's creature pick), as it always was here
	return s.hasConvoke(k, e.ctrl, e.card, e.obj, fh) || !improvise, improvise
}

// FeatConvokePicked reports whether o was already announced.
func (s *State) FeatConvokePicked(o Obj) bool { return s.convokePicked(o) }

// FeatConvokePickCount is the number of permanents announced so far.
func (s *State) FeatConvokePickCount() int { return s.convokePickCount() }

// FeatColorsOf is the battlefield permanent o's colours (bit i: W U B R G).
func (s *State) FeatColorsOf(o Obj) uint8 { return uint8(s.colorsAt(s.k(), s.objs[o].slot)) }

// payConvokeFromPool pays what the picks left of the cost from p's floating
// mana: the restricted batches the spell may spend first, then the plain pool,
// as payRestrSpend carves them, but with no planner taps, since the cast's
// window has already tapped the sources (gorge pays restricted mana the
// descriptor admits before ordinary mana, CR 106.6).
func (s *State) payConvokeFromPool(k *prep, e *stackEntry, red *kop.ManaCost, anyColor bool) {
	p := e.ctrl
	var m uint8
	if s.rbAny(p) {
		m = s.restrSpendable(k, p, e.card, restrOriginOfEntry(e))
	}
	if m == 0 {
		s.payFromPoolRider(k, p, red, 0, anyColor)
		return
	}
	var tk [maxRB][kop.NumManaKinds]uint8
	var rn [kop.NumManaKinds]uint8
	s.rbUnits(p, m, &rn)
	s.rbTake(p, m, &tk)
	var ord [kop.NumManaKinds]uint8
	for c := range ord {
		ord[c] = s.pl[p].pool[c] - rn[c] // the plain units beside the restricted ones taken into the pool
	}
	pre := s.pl[p].pool
	s.payFromPoolRider(k, p, red, 0, anyColor)
	s.rbSettle(p, &tk, &ord, &pre, s.rbTyped(p))
}
