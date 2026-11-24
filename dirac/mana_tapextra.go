package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Extra mana a tap brings: a StatTapExtra static (Groundchuck & Dirtbag,
// Badgermole Cub, Forsaken Monument: a Mode$ TapsForMana | Static$ True trigger
// adding one literal symbol) adds its mana, once, whenever its controller taps
// a matching permanent for mana. gorge folds it into the tapped source's
// production (rules payment_plan_interference.go paymentPlanTapObservers,
// pay planalts.go addTapExtra), so the plan prices the source at its mana plus
// the extra; here the extra is a fixed vector beside the source's colour
// choice, taken off the pips before any colour is picked (mana.go
// planner.fixedOff). Without a carrier in the table (genState.anyTapExtra)
// none of this is reached.

// StallTapExtra: a tap that gets extra mana from a source the planner models
// as one plain tap (a choice among colours beside {C} under a carrier limited to a
// {C} tap, a sacrificed, painful, restricted or chosen-colour source, a filter's or a
// gate's tap).
const StallTapExtra = 143

func init() { stallNames[StallTapExtra] = "tapextra" }

// srcExtra is the mana the controller's StatTapExtra statics add when the
// permanent at sl taps for mana through the ability pc describes. A
// source the extra cannot be modelled on stalls (StallTapExtra) and adds
// nothing.
//
// An all-at-once source's own rest (pcard.manaXv, mana_eachcolour.go) is
// part of it, carrier or none.
func (s *State) srcExtra(k *prep, sl Slot, pc *pcard) (xv [kop.NumManaKinds]uint8) {
	xv, hit := s.srcExtraHit(k, sl, pc)
	if hit && (!s.tapExtraPlain(pc) || pc.painCol != 0 || xv == [kop.NumManaKinds]uint8{}) {
		s.capFull(StallTapExtra)
		return [kop.NumManaKinds]uint8{}
	}
	addOwnXv(&xv, pc)
	return xv
}

// srcExtraPain is the planner's extra for a painful source (a painland's
// {C} ability and its painful coloured one, or a pain-only source): xc the
// extra of its normal ability, xp the extra of the painful one (srcExtraPainPick),
// r the part of it that is one mana of the pick (a reflected carrier over a
// several-type production: gorge addTapExtra adds per alternative, rules
// payment_plan_interference.go paymentPlanTapObservers), which the caller
// folds into the tap's amount. False, with the stall counted, when either
// depends on the pick in a way the caller cannot fold or the source is not
// one plain tap.
func (s *State) srcExtraPain(k *prep, sl Slot, pc *pcard) (xc, xp [kop.NumManaKinds]uint8, r uint8, ok bool) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return xc, xp, r, true
	}
	if !s.tapExtraPlainBase(pc) {
		if _, hit := s.srcExtraHit(k, sl, pc); hit {
			s.capFull(StallTapExtra)
			return xc, xp, r, false
		}
		return xc, xp, r, true
	}
	if !pc.painOnly {
		var hit bool
		if xc, hit = s.srcExtraHit(k, sl, pc); hit && xc == [kop.NumManaKinds]uint8{} {
			s.capFull(StallTapExtra)
			return xc, xp, r, false
		}
	}
	var stall bool
	r, xp, stall = s.srcExtraPainPick(k, sl, pc)
	if stall {
		s.capFull(StallTapExtra)
		return xc, xp, r, false
	}
	return xc, xp, r, true
}

// srcExtraPainPick is the extra a coloured pick of the painful alternative at
// sl brings: the reflected carriers' (StTapReflect) mana of the produced type
// and a literal carrier's fixed vector. A one-type production (pc.painCol's
// one colour, or {C} for a pain-only {C} source) makes the reflected extra
// that fixed vector (folded into xv); a several-type one makes it one mana of
// the pick (r; the caller folds it into the tap's amount: painUnit,
// addPainAvail, addExtraCol). A StTapOnC carrier reaches by the source's own
// mask, as srcExtraTapped reads it; a source whose own ability is a {C}-or-
// colour choice leaves the carrier's extra on the pick, which cannot be
// priced here (stall). False, with the stall counted, for that shape.
func (s *State) srcExtraPainPick(k *prep, sl Slot, pc *pcard) (r uint8, xv [kop.NumManaKinds]uint8, stall bool) {
	if k.gen == nil || !k.gen.anyTapExtra || pc.manaAb < 0 || pc.manaSacOnly ||
		pc.painCol == 0 || pc.manaXv != ([kop.NumManaKinds]uint8{}) || !s.tapExtraPlainBase(pc) {
		return r, xv, false
	}
	prod := uint8(pc.painCol) & 0x3f
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if !s.tapExtraReaches(k, st, sl, ctl) {
			return true
		}
		if a.StFlags&(kop.StTapReflect|kop.StTapOnC) != 0 {
			if a.StFlags&kop.StTapReflect != 0 {
				r += uint8(a.B)
				return true
			}
			// a StTapOnC carrier reaches by the source's own mask (talisman
			// beside Forsaken Monument), as srcExtraTapped reads it
			switch m := srcMask(pc); {
			case m&(1<<kop.C) == 0:
				return true
			case m != 1<<kop.C:
				stall = true // the extra depends on the pick ({C} or a colour)
				return true
			}
			xv[kop.C] += uint8(a.B)
			return true
		}
		xv[a.A] += uint8(a.B)
		return true
	})
	if r > 0 && prod != 0 && prod&(prod-1) == 0 {
		// one produced type: the extra is that fixed vector beside the tap
		xv[bits.TrailingZeros8(prod)] += r
		r = 0
	}
	return r, xv, stall
}

// srcExtraHit is srcExtra without the stall: the extra vector, and whether any
// StatTapExtra of the controller reaches a tap of sl for mana (the vector is
// empty when the extra depends on the pick).
func (s *State) srcExtraHit(k *prep, sl Slot, pc *pcard) (xv [kop.NumManaKinds]uint8, hit bool) {
	return s.srcExtraAlt(k, sl, pc, false)
}

// srcExtraAlt is srcExtraHit for the normal ability (colour false) or for a
// coloured pick (colour true: a StTapOnC carrier does not reach it).
func (s *State) srcExtraAlt(k *prep, sl Slot, pc *pcard, colour bool) (xv [kop.NumManaKinds]uint8, hit bool) {
	if pc.manaAb < 0 || pc.manaSacOnly {
		return xv, false // no tap: the trigger does not fire
	}
	return s.srcExtraTapped(k, sl, pc, colour)
}

// srcExtraTapped is srcExtraAlt for a tap that is known to happen (a filter's
// {T}): a granted or first-only ability leaves pcard.manaAb without the
// source's own tap, yet the permanent is still tapped for mana.
func (s *State) srcExtraTapped(k *prep, sl Slot, pc *pcard, colour bool) (xv [kop.NumManaKinds]uint8, hit bool) {
	ctl := s.perms[sl].ctrl
	eachPick := !colour && s.srcEachPick(k, sl, pc) == 1
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if !s.tapExtraReaches(k, st, sl, ctl) {
			return true
		}
		if a.StFlags&kop.StTapReflect != 0 {
			// one mana of the type the tap produced: fixed for a source of one
			// type, the pick's otherwise (and a filter's colour is picked after
			// its tap)
			if eachPick {
				// an all-at-once source's types come together: the extra is one
				// more mana of one of them (srcEachPick), no fixed vector
				return true
			}
			m := srcMask(pc)
			if !colour && m&(m-1) != 0 && pc.manaXv == [kop.NumManaKinds]uint8{} && s.tapExtraPlain(pc) {
				// one plain tap of several colours: srcReflectPick prices
				// the extra as one more of the pick (no fixed vector: not a hit)
				return true
			}
			if colour || m == 0 || m&(m-1) != 0 || pc.manaXv != [kop.NumManaKinds]uint8{} {
				hit = true
				xv = [kop.NumManaKinds]uint8{}
				return false
			}
			xv[bits.TrailingZeros8(m)] += uint8(a.B)
			hit = true
			return true
		}
		if a.StFlags&kop.StTapOnC != 0 {
			if colour {
				return true
			}
			switch m := srcMask(pc); {
			case m&(1<<kop.C) == 0:
				return true
			case m != 1<<kop.C:
				hit = true // a colourless-or-coloured choice: the extra depends on the pick
				xv = [kop.NumManaKinds]uint8{}
				return false
			}
		}
		xv[a.A] += uint8(a.B)
		hit = true
		return true
	})
	return xv, hit
}

// filtExtra is the extra a filter activation of sl brings: the filter taps sl
// for mana, so a StatTapExtra static adds its mana beside the filter's own
// unit (Badgermole Cub on an earthbent Rumble Arena's {1},{T}). A filter that
// costs no {T} (a once-each-turn one) taps nothing and brings none. A choice
// production's colour is picked after the tap, so a carrier limited to a {C}
// tap does not reach it; a fixed one-colour production's type is known.
func (s *State) filtExtra(k *prep, sl Slot) (xv [kop.NumManaKinds]uint8) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return xv
	}
	a := s.filtAb(k, sl)
	if a.Cost.Flags&kop.CostTap == 0 {
		return xv
	}
	// A reflect carrier's extra is one mana of the type the tap produced (CR
	// 106.12a): a fixed vector beside a production of one fixed colour, whose
	// amount may still be dynamic (Cabal Coffers' {B} for each Swamp beside
	// Mana Flare). A choice or all-at-once production's extra is the pick's,
	// so only a pick asks srcExtraTapped for the pick's colour; asking for it
	// beside a fixed production dropped the carrier's vector and the census
	// read one mana short (Mournwhelk's evoked {3}{B} beside the Coffers). A
	// plain Produced$ Any is a pick too (it lowers with no AfManaChoice): its
	// ManaColors is ColAny, and pricing it as a fixed colour would read the
	// card's first mana ability (Hidden Grotto's {C}) instead of the filter's
	// chosen colour.
	colour := a.ManaEach || a.AFlags&kop.AfManaChoice != 0 || bits.OnesCount8(uint8(a.ManaColors)&0x3f) != 1
	xv, _ = s.srcExtraTapped(k, sl, s.manaPCAt(k, sl), colour)
	if a.ManaEach && !DebugEachReflectNone {
		s.filtEachReflect(k, sl, a, &xv)
	}
	return xv
}

// filtReflectPick is the reflected extra (StTapReflect: Mana Flare) of a filter
// activation whose colour is picked after its tap (Starfleet Academy's
// {5},{T} any colour): one more mana of the pick, CR 106.12a. filtExtra leaves
// it out because the pick is unknown there, so the activation floats it
// beside its own mana. A fixed one-colour production already carries it in
// filtExtra, and an all-at-once one's is filtEachReflect's (filtExtra).
func (s *State) filtReflectPick(k *prep, sl Slot) (r uint8) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return 0
	}
	a := s.filtAb(k, sl)
	if a.Cost.Flags&kop.CostTap == 0 || a.ManaEach ||
		a.AFlags&kop.AfManaChoice == 0 && bits.OnesCount8(uint8(a.ManaColors)&0x3f) == 1 {
		return 0
	}
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		if s.tapExtraReaches(k, st, sl, ctl) && st.a.Static.StFlags&kop.StTapReflect != 0 {
			r += uint8(st.a.Static.B)
		}
		return true
	})
	return r
}

// DebugEachReflectNone drops the reflected extra of an all-at-once filter's tap
// (the mutation test: the games must diverge).
var DebugEachReflectNone bool

// filtEachReflect adds to xv the reflected extra of an all-at-once filter's tap
// (Sungrass Prairie's {G}{W} under Mirari's Wake: "add one mana of any type that
// land produced"). The pick is a mid-resolution ask gorge cannot pose inside a
// mana ability, so it keeps its first candidate (effManaReflected's "chose first
// reflected colour", ManaReflectedCandidates' W, U, B, R, G order): one more mana
// of the filter's first colour per reflect carrier reaching the tap.
func (s *State) filtEachReflect(k *prep, sl Slot, a *kop.Ability, xv *[kop.NumManaKinds]uint8) {
	first := -1
	for c, n := range filtEachVec(a) {
		if n != 0 && c < kop.C {
			first = c
			break
		}
	}
	if first < 0 {
		return
	}
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		if sa := &st.a.Static; s.tapExtraReaches(k, st, sl, ctl) && sa.StFlags&kop.StTapReflect != 0 {
			xv[first] += uint8(sa.B)
		}
		return true
	})
}

// srcExtraPick is the extra a tap of sl through the ability pc brings when it
// makes colour col: a carrier limited to a {C} tap reaches only a {C} pick. A
// source the extra cannot be priced on (not one plain tap) stalls, as srcExtra.
func (s *State) srcExtraPick(k *prep, sl Slot, pc *pcard, col int) (xv [kop.NumManaKinds]uint8) {
	if pc == nil || pc.manaAb < 0 || pc.manaSacOnly {
		return xv
	}
	ctl := s.perms[sl].ctrl
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if !s.tapExtraReaches(k, st, sl, ctl) {
			return true
		}
		if a.StFlags&kop.StTapOnC != 0 && col != kop.C {
			return true
		}
		if a.StFlags&kop.StTapReflect != 0 {
			xv[col] += uint8(a.B) // the type this tap made
			hit = true
			return true
		}
		xv[a.A] += uint8(a.B)
		hit = true
		return true
	})
	if hit && !s.tapExtraPlain(pc) {
		s.capFull(StallTapExtra)
		return [kop.NumManaKinds]uint8{}
	}
	addOwnXv(&xv, pc)
	return xv
}

// DebugTapChosenOff drops the Produced$ Chosen extra (Utopia Sprawl) at the tap
// (mutation test only).
var DebugTapChosenOff bool

// DebugTapAnyColOff drops the any-colour tap extra (Fertile Ground) at the tap
// (mutation test only).
var DebugTapAnyColOff bool

// tapExtraReaches reports whether st is a StatTapExtra static that adds its
// mana when the permanent at sl, controlled by ctl, taps for mana: its
// controller's own taps, or every player's under StTapAnyPlayer (the extra
// then goes to the tapping player, whose pool the caller fills).
//
// A Cond2-gated static (Rikala, Homarid King), a chosen-colour or any-colour
// extra and a count-scaled one (StTapDynamic: Elvish Guidance) never reach
// here: the planner's pricing readers skip it, because gorge's planner cannot
// fold the trigger, so its census unit and its potential bound
// (paymentPlanTapExtraMana, whole=true) read the tap without the extra. The
// extra floats at the tap itself (addGatedExtra), exactly the trigger's
// resolution.
func (s *State) tapExtraReaches(k *prep, st statSrc, sl Slot, ctl uint8) bool {
	a := &st.a.Static
	if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || st.sl == NoSlot ||
		a.Cond2 != 0 || a.StFlags&(kop.StTapChosen|kop.StTapAnyCol|kop.StTapDynamic) != 0 ||
		st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 {
		return false
	}
	if !s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
		return false
	}
	s.tapExtraEchoStall(k, st)
	return true
}

// addGatedExtra floats the Cond2-gated, chosen-colour, any-colour and
// count-scaled StatTapExtra statics of sl's tap in p's pool: the extra an
// unfoldable TapsForMana trigger adds as it resolves at the tap (Rikala,
// Homarid King; a Produced$ Any/Combo Any or Produced$ Chosen body; Elvish
// Guidance's Count$Valid Elf), while the planner's readers never price it
// (tapExtraReaches) and the tap itself, which gorge fires the trigger on,
// still brings it. The any-colour ask's answer is the scratch fillTapAnyCol
// stored: gorge's WindowChoices answer for the same owed pips.
func (s *State) addGatedExtra(k *prep, p uint8, sl Slot, ctl uint8) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	ord := 0 // the reaching Produced$ Any carriers so far: a search's forced answers are per carrier (forceAnyColAnswers)
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		chosen := a.StFlags&kop.StTapChosen != 0   // Produced$ Chosen: gated like a Cond2 static, in the source's colour
		anyCol := a.StFlags&kop.StTapAnyCol != 0   // Produced$ Any / Combo Any: the answer rides genState (fillTapAnyCol)
		dynamic := a.StFlags&kop.StTapDynamic != 0 // StTapDynamic: a Count$ value, read here
		if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || st.sl == NoSlot || a.Cond2 == 0 && !chosen && !anyCol && !dynamic ||
			st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 {
			return true
		}
		if a.StFlags&(kop.StTapReflect|kop.StTapOnC) != 0 {
			s.capFull(StallTapExtra) // the gated lowering refuses those flags
			return true
		}
		if !s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		if (chosen || anyCol || dynamic) && (DebugTapChosenOff && chosen || DebugTapAnyColOff && anyCol ||
			s.pool != nil && s.pool.census) {
			return true // an unless / ward window's census prices the deferred source's own mana, never the trigger's extra (gorge unlessWindowUnits)
		}
		if a.Cond2 != 0 && !s.condEval(k, a.Cond2, evalCtx{src: s.perms[st.sl].obj, srcSl: st.sl, ctrl: st.ctrl}) {
			return true // the gate holds only while the condition does
		}
		s.tapExtraEchoStall(k, st)
		if anyCol {
			s.addAnyColExtra(p, a, ord)
			ord++
			return true
		}
		if dynamic {
			n := s.evalV(k, a.VA, evalCtx{src: s.perms[st.sl].obj, srcSl: st.sl, ctrl: st.ctrl})
			if n > 0 {
				s.pl[p].pool[int(a.A)] += uint8(min(n, 255))
			}
			return true
		}
		col := int(a.A)
		if chosen {
			if ch := s.perms[st.sl].chosen; ch < 5 {
				col = int(ch)
			} else {
				return true // no colour was chosen: gorge's Produced$ Chosen adds nothing
			}
		}
		s.pl[p].pool[col] += uint8(a.B)
		return true
	})
}

// anyColExtraScratch is the pending any-colour tap extra's answer, in unit
// order: genState.winCol carries the first (or only) unit's colour, and the
// first two reserved bytes the rest (a Combo of at most three units, kgen's
// bound). It is per-resolution scratch, never rules state.
func (s *State) anyColExtraColour(u int) uint8 {
	switch u {
	case 1:
		return s.g.rsv[1]
	case 2:
		return s.g.rsv[2]
	}
	return s.g.winCol
}

// addAnyColExtra adds a's mana to p's pool from the answer fillTapAnyCol
// stored: one colour for all B mana (Produced$ Any), or B units each their
// own colour (Produced$ Combo Any). A search that answered each carrier's ask
// itself (forceAnyColAnswers) gives the ord-th carrier its own answer.
func (s *State) addAnyColExtra(p uint8, a *kop.Static, ord int) {
	if a.A == 1 { // Combo Any: one unit per answer
		for u := 0; u < int(a.B); u++ {
			s.pl[p].pool[s.anyColExtraColour(u)%5]++
		}
		return
	}
	col := s.g.winCol // Any: one colour for the whole amount
	if s.g.rsv[anyColForcedAt] != 0 {
		col = s.anyColExtraColour(ord)
	}
	s.pl[p].pool[col%5] += uint8(a.B)
}

// fillTapAnyCol sets the pending any-colour tap extra's answer for a tap of sl
// against the play a. The answer is the first one gorge's prefix search tries
// (anyColFirst); deferGapSearch's second run, where that line prices nothing,
// answers by what the play still owes (anyColOwed).
func (s *State) fillTapAnyCol(k *prep, p uint8, sl Slot, a Action, ownCol, ownN int) {
	if k == nil || k.gen == nil || !k.gen.anyTapExtra || a == 0 {
		return
	}
	if s.anyColOwedRun() {
		s.anyColOwed(k, p, sl, a, ownCol, ownN)
		return
	}
	need := s.anyColNeed(k, a)
	s.eachAnyColStatic(k, sl, func(units int) [3]uint8 { return anyColFirst(&need, units) })
}

// anyColNeed is the colours play a's printed base cost names (gorge's
// scriptNeedColours, playNeed). A hybrid pip the build has already announced
// reads the announced face instead of both faces: gorge's ManaPayment.Owed
// holds the resolved pip.
func (s *State) anyColNeed(k *prep, a Action) [kop.NumManaKinds]bool {
	need := s.playNeed(k, a)
	if s.buildLive() {
		if m, _, ok := s.playBaseCost(k, a); ok && m.Hyb[0] != 0 {
			for i, h := range m.Hyb {
				if h == 0 {
					break
				}
				first, second := hybColours(m, i)
				need[first], need[second] = false, false
				if int(s.bld().pipN) > i {
					need[s.bld().pipCol(uint8(i))] = true
				} else {
					need[first], need[second] = true, true
				}
			}
		}
	}
	return need
}

// anyColFirst is the first answer of an any-colour extra's ask in gorge's
// prefix search: the ask's answers are the colour combinations in option
// order, and scriptOrderAnswers moves the ones naming a colour the play's
// printed cost asks for ahead of the rest, so the first is the lowest
// combination holding a needed colour: W in every unit but the last, which
// takes the lowest needed colour (W when it is needed, or when none is). A
// {B}{B} cast's Swamp under Dawn's Reflection floats {B} and answers {W}{B},
// as a {R} ability's Mountain answers {W}{R}, whatever the pool holds (pool
// dr-mbc-abbb pair 77, decision 287; tapsdefer_anycol_extra_activation).
func anyColFirst(need *[kop.NumManaKinds]bool, units int) (cols [3]uint8) {
	if units < 1 || units > len(cols) {
		units = len(cols)
	}
	if need[0] {
		return cols
	}
	for c := 1; c < 5; c++ {
		if need[c] {
			cols[units-1] = uint8(c)
			break
		}
	}
	return cols
}

// anyColOwed is the second answer: each unit the first pip play a still owes
// in W U B R G order, else the lowest colour, the owed pips being the play's
// cost less the pool the tap's own mana has already reached (the tap's mana
// reaches the pool before the triggered ability resolves). A Combo unit comes
// off the owed count as it is answered. A kicked cast owes the kicker
// payment's pips too (tapAnyColPips).
func (s *State) anyColOwed(k *prep, p uint8, sl Slot, a Action, ownCol, ownN int) {
	var owed [kop.NumManaKinds]uint8
	if cnt, ok := s.tapAnyColPips(k, a); ok {
		owed = cnt // exact per-colour pips, e.g. a Combo Any amount of one colour
	} else {
		// a hybrid, twobrid or otherwise reduced cost: the play's need is the
		// colour list gorge's ask order reads, so the first named pip is the
		// ask's first owed colour
		for c, on := range s.anyColNeed(k, a) {
			if on {
				owed[c] = 1
			}
		}
	}
	for c := range owed {
		n := int(owed[c]) - int(s.pl[p].pool[c])
		if c == ownCol {
			n -= ownN
		}
		if n < 0 {
			n = 0
		}
		owed[c] = uint8(n)
	}
	s.fillTapAnyColOwed(k, p, sl, &owed)
}

// tapAnyColPips is the any-colour tap extra's owed pips: playCostCounts
// except that a generic part does not hide the coloured pips. gorge's
// standalone colour ask reads the announced pips (windowOwed), and a generic
// pip takes any colour, so Dawn's Reflection's answer to a {2}{B}{B} cast is
// still driven by the two black pips. A flexible pip (X, snow, hybrid,
// Phyrexian, twobrid) keeps the playNeed fallback: its announced face is not
// the printed pips.
func (s *State) tapAnyColPips(k *prep, a Action) (cnt [kop.NumManaKinds]uint8, ok bool) {
	m, f, base := s.playBaseCost(k, a)
	if !base && s.anyColOwedRun() && s.kickedArmOffer(k, a) {
		// A kicked cast announces the printed pips and the kicker payment's,
		// the total CR 601.2f composes. Bloodchief's Thirst kicked owes
		// {B}{B}, so a Swamp under Dawn's Reflection answers {B}{W}, where
		// the first answer (a kicked cast reads no pips, gorge's
		// scriptPlayBaseCost) is {W}{W}: gorge's search goes on to the
		// answer that pays the kicked offer (pool dr-mbc-abbb pair 77,
		// decision 290).
		cd := &k.t.Cards[s.objs[Obj(a.Source())].card]
		m, base = addCost(cd.Cost, kickerPayment(cd)), true
	}
	if !base || m.X != 0 || m.Snow != 0 || m.Hyb[0] != 0 || m.Tb != 0 || f&kop.CostForage != 0 {
		return cnt, false
	}
	for c, n := range m.Pips {
		if n > 0 {
			cnt[c], ok = n, true
		}
	}
	return cnt, ok
}

// fillTapAnyColCost is fillTapAnyCol for a payment window's explicit cost. col
// and n are the mana the tap itself makes (poolAdd's arguments, which follow).
func (s *State) fillTapAnyColCost(k *prep, p uint8, sl Slot, cost *kop.ManaCost, col, n int) {
	if k == nil || k.gen == nil || !k.gen.anyTapExtra || cost == nil {
		return
	}
	if owed, ok := s.announcedPipsOwed(k, p, col, n); ok {
		s.fillTapAnyColOwed(k, p, sl, &owed)
		return
	}
	// The ask is answered before the tap's own mana reaches the pool: its
	// demand is windowCol's owed cost, the snapshot gorge's WindowChoices
	// reads (ManaPayment.Owed), not cost less the pool after the tap.
	var due kop.ManaCost
	s.windowOwedCost(k, p, cost, &due)
	s.fillTapAnyColOwed(k, p, sl, &due.Pips)
}

// announcedPipsOwed is the demand of an any-colour extra's ask in a hybrid
// cast's manual window (CR 601.2g): the oracle's colour ask of a window with
// no ManaPayment owes the hybrid faces the caster announced (the resolved
// pips, not both printed halves and not the cost's fixed pips) less the pool,
// and the ask comes after the tap's own mana n of col reached it (the tap's
// TapsForMana trigger resolves with that mana floating). False when the build
// has announced no hybrid face (a twobrid's faces are a mask, never read here).
func (s *State) announcedPipsOwed(k *prep, p uint8, col, n int) (owed [kop.NumManaKinds]uint8, ok bool) {
	if !s.buildLive() || s.bld().pipN == 0 {
		return owed, false
	}
	if h := s.buildCost0(k).Hyb[0]; h == 0 || h == kop.HybTwobrid {
		return owed, false
	}
	pool := s.pl[p].pool
	if col >= 0 && col < len(pool) {
		pool[col] += uint8(n)
	}
	for i := uint8(0); i < s.bld().pipN; i++ {
		if c := s.bld().pipCol(i); c < len(owed) {
			owed[c]++
		}
	}
	for c := range owed {
		owed[c] -= min(owed[c], pool[c])
	}
	return owed, true
}

// fillTapAnyColOwed is fillTapAnyCol's shared answer: every any-colour
// StatTapExtra static reaching sl gets its units filled (normally one).
func (s *State) fillTapAnyColOwed(k *prep, p uint8, sl Slot, owed *[kop.NumManaKinds]uint8) {
	s.eachAnyColStatic(k, sl, func(units int) (cols [3]uint8) {
		for u := 0; u < units && u < len(cols); u++ {
			cols[u] = uint8(anyColOwedColour(owed))
		}
		return cols
	})
}

// eachAnyColStatic stores the answer pick names for the first any-colour
// StatTapExtra reaching sl: the colours of its units (one for a Produced$ Any
// amount, whatever the amount; B for a Combo Any), the carrier the add arm
// reads (addAnyColExtra).
func (s *State) eachAnyColStatic(k *prep, sl Slot, pick func(units int) [3]uint8) {
	ctl := s.perms[sl].ctrl
	s.g.rsv[anyColForcedAt] = 0 // the deterministic fill: no search answers (forceAnyColAnswers)
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || a.StFlags&kop.StTapAnyCol == 0 ||
			s.pool != nil && s.pool.census || st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 ||
			!s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		units := 1
		if a.A == 1 {
			units = int(a.B)
		}
		cols := pick(units)
		s.g.winCol = cols[0]
		for u := 1; u < len(cols); u++ {
			s.g.rsv[u] = cols[u]
		}
		return false // the first reaching static is the carrier the add arm reads: gorge answers each carrier's ask against the same owed pips
	})
}

// anyColOwedColour takes one unit's colour off owed: the first pip still owed
// (W U B R G), else {W}, exactly payexec.WindowAnswer's answer.
func anyColOwedColour(owed *[kop.NumManaKinds]uint8) int {
	for c := 0; c < 5; c++ {
		if owed[c] > 0 {
			owed[c]--
			return c
		}
	}
	return 0
}

// tapExtraEchoStall stalls (StallTapExtra) when a Panharmonicon static echoes
// the TapsForMana trigger st lowers (Annie Joins Up beside Roxanne, Starfall
// Savant, CR 603.2d): gorge resolves the trigger once per placement (rules
// statics_echo.go), so the tap floats one more extra per echo that its planner
// does not price (paymentPlanTapObservers folds it once), and the kernel's one
// fixed extra models neither half.
func (s *State) tapExtraEchoStall(k *prep, st statSrc) {
	if k.gen == nil || !k.gen.anyPanharm {
		return
	}
	l := trigListener{obj: s.perms[st.sl].obj, sl: st.sl, ctrl: st.ctrl, card: s.cardAt(st.sl)}
	if s.panharmEchoes(k, l, kop.TrigTaps, NoObj, st.ctrl) > 0 {
		s.capFull(StallTapExtra)
	}
}

// subXv takes a plan type's fixed extra back out of p's pool.
func (s *State) subXv(p uint8, xv *[kop.NumManaKinds]uint8) {
	for c := range xv {
		s.pl[p].pool[c] -= xv[c]
	}
}

// tapExtraPlain: the source is one plain {T} ability whose mana is n of one
// fixed colour or one colour of a choice.
func (s *State) tapExtraPlain(pc *pcard) bool {
	return s.tapExtraPlainBase(pc) && pc.painCol == 0
}

// tapExtraPlainBase is tapExtraPlain without the painful alternative: the
// planner prices that one itself (srcExtraPain). The extra is a fixed vector
// whatever the amount (an evaluated one, all of it one colour) and whichever of
// several abilities taps (gorge adds it to each alternative, planalts.go
// appendUnitAlternatives), so those shapes carry it.
func (s *State) tapExtraPlainBase(pc *pcard) bool {
	return pc.manaTap && !pc.manaSac && !pc.manaChosen && pc.restrU == 0 && pc.rankCol == 0
}

// tapExtraStall stalls when sl's tap would have brought extra mana through a
// path that does not add it (a filter's, a gate's, a sacrificed source's).
func (s *State) tapExtraStall(k *prep, sl Slot) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	pc := s.manaPCAt(k, sl)
	if s.srcEachPick(k, sl, pc) > 0 {
		// the pick floats unasked (nothing poses its colour ask yet): every
		// reader of srcExtra outside the census builds and the plan drops it
		// silently, so the tap refuses the shape as base's srcExtra stall did
		s.capFull(StallTapExtra)
		return
	}
	if x := s.srcExtra(k, sl, pc); x != pc.manaXv {
		s.capFull(StallTapExtra)
	}
}

// srcReflectPick is the mana the reflected StatTapExtra carriers (StTapReflect)
// reaching the tap of sl add of the colour that tap picks, for a source that is
// one plain single-type-per-tap unit of several colours (a dual land, an
// any-colour source): the extra is one more of the pick, so the unit makes
// manaN+r of it (gorge folds the extra into each single-colour production,
// pay.PaymentPlanProductions). Zero for every other source: a one-type source
// carries it as a fixed vector (srcExtraTapped), an all-at-once, sacrificed,
// restricted or painful one stalls.
func (s *State) srcReflectPick(k *prep, sl Slot, pc *pcard) (r uint8) {
	if k.gen == nil || !k.gen.anyTapExtra || pc == nil || pc.manaAb < 0 || pc.manaSacOnly ||
		pc.manaXv != [kop.NumManaKinds]uint8{} || !s.tapExtraPlain(pc) {
		return 0
	}
	if m := srcMask(pc); m&(m-1) == 0 {
		return 0
	}
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		if a := &st.a.Static; a.StFlags&kop.StTapReflect != 0 && s.tapExtraReaches(k, st, sl, ctl) {
			r += uint8(a.B)
		}
		return true
	})
	return r
}

// srcEachPick is the mana the reflected StatTapExtra carriers (StTapReflect)
// reaching the tap of sl add beside an all-at-once source's whole production
// (Azorius Chancery's {W}{U}, mana_eachcolour.go): the extra is one mana of ONE
// of the types the tap produced (CR 106.12a), asked at the tap -- gorge's
// planner names no one type over the several-type production, so its census
// defers the source and the prefix script activates it for real, the carrier's
// colour ask resolving in the same batch (engine/rules mana_activation.go
// resolveTriggeredManaAbilities). Zero for every other source (srcReflectPick
// prices a choice source's; a one-type, sacrificed, restricted or painful one
// stalls as srcExtra reads it).
func (s *State) srcEachPick(k *prep, sl Slot, pc *pcard) (r uint8) {
	if k.gen == nil || !k.gen.anyTapExtra || pc == nil || pc.manaAb < 0 || pc.manaSacOnly ||
		pc.manaXv == [kop.NumManaKinds]uint8{} || !s.tapExtraPlain(pc) {
		return 0
	}
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		if a := &st.a.Static; a.StFlags&kop.StTapReflect != 0 && s.tapExtraReaches(k, st, sl, ctl) {
			r += uint8(a.B)
		}
		return true
	})
	return r
}

// eachPickMask is the types an all-at-once source's tap produces: the colours
// its pick (srcEachPick) may name.
func eachPickMask(pc *pcard) uint8 { return uint8(pc.manaCol)&0x3f | xvMask(pc) }

// addEachPick adds the census unit of an all-at-once source's reflected pick:
// one more mana of one of the produced types, beside the production's own units
// (the unit's colour, srcExtra's rest). A single pick is one unit of the
// produced mask; two carriers' picks are independent asks srcExtra stalls on.
func (s *State) addEachPick(k *prep, a *avail, sl Slot, pc *pcard) {
	if s.srcEachPick(k, sl, pc) == 1 {
		a.addUnits(eachPickMask(pc), 1)
	}
}

// xvTotals is the colours and the total of a plan type's fixed extra.
func xvTotals(xv *[kop.NumManaKinds]uint8) (xm, xt uint8) {
	for c, v := range xv {
		if v > 0 {
			xm |= 1 << c
			xt += v
		}
	}
	return xm, xt
}

// planEachPick turns the plan unit of an all-at-once source under a reflect
// carrier into the unit its tap makes (srcEachPick): the production is fixed
// ({W}{U}: the unit's own colour joins the rest in xv), and the tap adds one
// mana of one of its types, which the search picks per need. It reports whether
// the source is such a unit.
func (s *State) planEachPick(k *prep, sl Slot, pc *pcard, xv *[kop.NumManaKinds]uint8, xm, xt, tn, mask, dmk *uint8) bool {
	if pc.painCol != 0 || s.srcEachPick(k, sl, pc) != 1 {
		return false
	}
	col := bits.TrailingZeros8(uint8(pc.manaCol))
	xv[col] += pc.manaN
	*xm |= 1 << col
	*xt += pc.manaN
	*mask = eachPickMask(pc)
	*dmk, *tn = *mask, 1
	return true
}

// addXvPick floats a plan type's fixed extra in p's pool. The pick of an
// all-at-once source's reflected extra (utype.pick) is the controller's to
// make (reflectPickPark): it reports whether the plan stops for that ask.
func (s *State) addXvPick(p uint8, ty *utype, col int) bool {
	s.addXv(p, &ty.xv)
	return ty.pick && s.reflectPickPark(p, ty, col)
}

// srcYieldAt is srcYield plus the pick's reflected extra (srcReflectPick): the
// census unit of a source's tap.
func (s *State) srcYieldAt(k *prep, sl Slot, pc *pcard) uint8 {
	return srcYield(pc) + s.srcReflectPick(k, sl, pc)
}

// addReflectOne registers the one-colour limit of a source whose tap a
// reflected StatTapExtra carrier widens: its tap is srcYieldAt mana of the ONE
// colour picked (CR 106.12a; gorge's planner prices the extra into each
// declared production, pay planalts.go addTapExtra, and the census prices the
// widened unit as a one-colour record beside its backing, legal.go
// potUnitsAvail), not independent units of the union. The entry rides the
// group srcMaskAt names, so removeSrc's remove of that group at srcYieldAt
// drops it (mana.go dropOneColour), as it does for the census's record.
// manaOne sources already registered theirs. A mask with the {C} bit keeps the
// independent read: the one-colour entries price coloured picks only (ColAny's
// own limit, mana.go addOneColour), so an entry over a {C}-bearing mask would
// drop the pick the independent units still make.
func (s *State) addReflectOne(k *prep, a *avail, sl Slot, pc *pcard) {
	if pc.manaOne || s.srcReflectPick(k, sl, pc) == 0 && !dynPairOne(pc) {
		return
	}
	m := s.srcMaskAt(pc, sl)
	if m&(m-1) == 0 || m&^uint8(kop.ColAny) != 0 {
		return // one fixed colour: the carrier's extra is a fixed vector (srcExtra)
	}
	s.addOneColourMask(a, m, s.srcYieldAt(k, sl, pc))
}

// addExtra floats the extra mana of sl's tap in p's pool. A reflected extra of
// a several-colour source depends on the pick this caller does not know.
func (s *State) addExtra(k *prep, p uint8, sl Slot, pc *pcard) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	if s.srcReflectPick(k, sl, pc) > 0 || s.srcEachPick(k, sl, pc) > 0 {
		s.capFull(StallTapExtra)
	}
	xv := s.srcExtra(k, sl, pc)
	for c := range xv {
		s.pl[p].pool[c] += xv[c]
	}
	s.addGatedExtra(k, p, sl, s.perms[sl].ctrl)
}

// addExtraPick is addExtra for a tap whose colour col the caller knows: a
// painful source's extra depends on the pick (addExtraCol prices it), any
// other source's does not.
func (s *State) addExtraPick(k *prep, p uint8, sl Slot, pc *pcard, col int) {
	if pc.painCol != 0 {
		s.addExtraCol(k, p, sl, pc, col)
		return
	}
	s.addExtra(k, p, sl, pc)
}

// addExtraCol is addExtra for a tap that made colour col: a painful source's
// coloured pick brings only the extra of the carriers not limited to a {C}
// tap, its {C} ability all of them (srcExtraPain).
func (s *State) addExtraCol(k *prep, p uint8, sl Slot, pc *pcard, col int) {
	s.addGatedExtra(k, p, sl, s.perms[sl].ctrl) // the gated extra does not depend on the pick
	if pc.painCol == 0 {
		if k.gen == nil || !k.gen.anyTapExtra {
			return
		}
		xv := s.srcExtra(k, sl, pc)
		for c := range xv {
			s.pl[p].pool[c] += xv[c]
		}
		if r := s.srcReflectPick(k, sl, pc); r > 0 {
			s.poolAdd(p, col, int(r))
		}
		if s.srcEachPick(k, sl, pc) > 0 {
			// Stays a stall in every walk, the cast window's too: gorge's
			// ManaReflected ask (all options Kind "mana") is answered by the
			// oracle harness internally (oracle gorge.go internal), never a
			// canonical decision, so the kernel must not pose it
			// (TestEachReflectedPickIsNotPosedInTheWindow).
			s.capFull(StallTapExtra)
		}
		return
	}
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xc, xp, r, ok := s.srcExtraPain(k, sl, pc)
	if !ok {
		return
	}
	if pc.painOnly || uint8(pc.manaCol)&(1<<col) == 0 {
		xc = xp
		if r > 0 {
			// a several-type production's reflected extra is one mana of the
			// pick (srcExtraPainPick): this tap made col
			s.poolAdd(p, col, int(r))
		}
	}
	s.addXv(p, &xc)
}

// addXv floats a plan type's fixed extra in p's pool.
func (s *State) addXv(p uint8, xv *[kop.NumManaKinds]uint8) {
	for c := range xv {
		s.pl[p].pool[c] += xv[c]
	}
}

// removeSrc takes out of a the mana the permanent at sl would make by tapping
// for mana through pc, the extra its tap brings included: a cost that taps the
// source leaves it neither.
func (s *State) removeSrc(k *prep, a *avail, sl Slot, pc *pcard) {
	if s.availLeavesOut(k, sl) {
		return // availFor never counted it: the units standing are the pool's
	}
	noGrantWalk := k.gen != nil && s.g.noGrant && s.g.availX&availPlain != 0
	if noGrantWalk && s.manaGranted(k, sl) {
		return // an activated ability's prefix search leaves a granted-only source out of a (availFor): nothing of it to take out
	}
	if k.gen != nil && k.gen.anyContGrant && !noGrantWalk {
		var gi [maxGrants]int16
		if s.grantIdxs(k, sl, &gi) > 0 {
			// a source with a granted mana ability is one unit of its own
			// and the granted colours together (availGrant): taking out its
			// own mask alone would leave that unit standing, and the tap
			// that pays the cost would fund the cost's mana too. A noGrant
			// walk left the grant out of the census (availFor), so the
			// source stands as its printed unit and that is what comes out
			// (CR 602.2b, 601.2g-h).
			var u avail
			u.xd, u.xdOn = a.xd, a.xdOn
			s.availGrant(k, sl, pc, &u)
			for i := 0; i < u.n; i++ {
				a.removeWide(u.mask[i], u.cnt[i])
			}
			return
		}
	}
	if k.painMana && pc.painCol != 0 {
		s.removePainAvail(k, a, sl, pc)
		return
	}
	a.remove(s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc))
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xv := s.srcExtra(k, sl, pc)
	for c := range xv {
		if xv[c] > 0 {
			a.remove(1<<c, xv[c])
		}
	}
	if s.srcEachPick(k, sl, pc) == 1 {
		a.remove(eachPickMask(pc), 1)
	}
}

// filterOwnTakes is removeSrc for a filter walk, reporting whether the filter
// may still produce: true when the census owed nothing of the source's own
// tap (availLeavesOut, a no-grant walk's granted-only source, a production
// with no units) or took it out of a, false when the census counted the own
// production and it was already gone from a -- an earlier fee of the same
// window spent the mana the permanent's one tap made (CR 605.1a), so the
// permanent cannot tap again to make this filter's mana. A filter walk that
// ignores the false return double-counts the tap: it kept Fetid Heath's {C}
// standing as another filter's fee while Heath's {W/B} ability still made its
// two (tail-01 pair 122: The Mana Rig's X ask read X = 3 off a board whose
// window reaches 8 mana).
func (s *State) filterOwnTakes(k *prep, a *avail, sl Slot, pc *pcard) bool {
	if pc == nil || s.srcYieldAt(k, sl, pc) == 0 {
		return true // the census owed nothing of the source's own tap
	}
	if s.availLeavesOut(k, sl) {
		return true // the census never counted it
	}
	noGrantWalk := k.gen != nil && s.g.noGrant && s.g.availX&availPlain != 0
	if noGrantWalk && s.manaGranted(k, sl) {
		return true // the census never counted it either (removeSrc's early return above)
	}
	before := a.total
	s.removeSrc(k, a, sl, pc)
	return a.total < before
}

// removePainAvail takes out of a the units addPainAvail added for the painful
// source at sl: its pain-widened mask (painAvailMask, which holds the colours
// of its life-paid ability beside the printed ones), not srcMaskAt's printed
// one, which names no group of a. A Haunted Screen reserved as a tap candidate
// no longer makes the {G} its life-paid ability adds (CR 601.2g-h). The modal
// case addPainAvail holds out or counts in nModal is left as it stands.
func (s *State) removePainAvail(k *prep, a *avail, sl Slot, pc *pcard) {
	p := s.perms[sl].ctrl
	m := s.painAvailMask(pc, p)
	n := uint8(1)
	if pc.painOnly {
		n = max(pc.manaN, 1)
	}
	if m != 0 && k.gen != nil && k.gen.anyTapExtra {
		if _, _, r, ok := s.srcExtraPain(k, sl, pc); ok && r > 0 && (pc.painOnly || m&^(1<<kop.C) != 0) {
			n += r // the reflected extra rode the unit's amount (addPainAvail)
		}
	}
	if m != 0 && !(a.pain.on && s.removeTightPain(k, a, sl, pc, m)) {
		a.remove(m, n)
	}
	if m == 0 || k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xc, xp, _, ok := s.srcExtraPain(k, sl, pc)
	if !ok {
		return
	}
	both := m&(1<<kop.C) != 0
	colours := m&^(1<<kop.C) != 0
	for c := range xc {
		n := xp[c]
		if both && !colours {
			n = xc[c]
		}
		if n > 0 {
			a.remove(1<<c, n)
		}
	}
}

// removeTightPain takes the painful source out of a's painTight groups, where
// its units live once the summed lethal guard binds: the mask/cnt census holds
// only the group's normal ability (prim) and its modal extra, so the removal by
// painAvailMask above would find nothing, and the activation whose {T} taps the
// source would still price its own cost's mana with it (Centaur Garden's
// "{G}, {T}, Sacrifice" at two life). It reports false when no group holds the
// source, leaving the caller's removal to run.
func (s *State) removeTightPain(k *prep, a *avail, sl Slot, pc *pcard, m uint8) bool {
	pa := &a.pain
	pm := uint8(pc.painCol) & 0x1f
	prim := uint8(pc.manaCol) & 0x3f
	var ob uint8
	if pc.painOnly {
		ob = 1
		prim = 0
	}
	var ec uint8
	if a.nModal != 0 && m&^(1<<kop.C) != 0 {
		ec = s.painExtraC(k, sl, pc)
	}
	for g := uint8(0); g < pa.n; g++ {
		if pa.mask[g] != pm || pa.prim[g] != prim || pa.only>>g&1 != ob || pa.ec[g] != ec || pa.w[g] != pc.painN || pa.cnt[g] == 0 {
			continue
		}
		pa.cnt[g]--
		if ob == 0 {
			a.remove(prim, 1)
			if ec != 0 {
				a.remove(1<<kop.C, ec)
			}
		}
		return true
	}
	return false
}

// addPainAvail adds a painful source's units to a: its production mask (the
// offer gate's, painAvailMask) and the extra its tap brings. The extra of a
// coloured pick is the carriers not limited to a {C} tap; the {C} ability
// gets all of them. A source offering both makes either {C} and the {C}-only
// extra, or a colour: units cannot say that, so with a {C}-only extra it is
// counted as a modal source (avail.nModal) that painTight splits by group,
// where the board allows it (modal), else the table stalls.
func (s *State) addPainAvail(k *prep, sl Slot, pc *pcard, p uint8, a *avail, ms *modalSet, tight bool) {
	m := s.painAvailMaskAt(k, sl, pc, p)
	n := uint8(1)
	if pc.painOnly {
		n = max(pc.manaN, 1) // a pain-only source's whole activation (Ancient Tomb's {C}{C})
	}
	var xc, xp [kop.NumManaKinds]uint8
	r, ok := uint8(0), true
	if m != 0 && k.gen != nil && k.gen.anyTapExtra {
		xc, xp, r, ok = s.srcExtraPain(k, sl, pc)
	}
	both := m&(1<<kop.C) != 0
	colours := m&^(1<<kop.C) != 0
	if !colours && !pc.painOnly {
		r = 0 // only the {C} ability is reachable (a life too low for the pain): its extra is xc's, no pick is made
	}
	// a painland's {C} ability and its coloured pick both make 1+r of one type
	// when the reflected extra is the only thing the {C} ability's extra holds
	// beyond the pick's (reflectOneType): the source is one tap of 1+r mana of
	// one type, {C} or a colour of its mask
	oneType := r > 0 && ok && both && colours && !pc.painOnly && reflectOneType(&xc, &xp, r)
	if r > 0 {
		// a reflected extra of a several-type production is one mana of the
		// pick: the unit's amount carries it, and the one-colour limit keeps
		// the bound exact (a {W} and a {U} are not payable from one tap). Only
		// the whole-mask pain-only unit and the one-type painland carry it;
		// anything narrower or modal-priced asks the pick in a shape the
		// bound cannot pose.
		whole := pc.painOnly && m == uint8(kop.ColAny)
		if !whole && !oneType || s.g.availX&(availAny|availWin|availWinCol) != 0 || k.gen.anyContGrant && !oneType {
			s.capFull(StallTapExtra)
			return
		}
		n += r
	}
	if m != 0 {
		a.addUnits(m, n)
		switch {
		case oneType:
			// the {C} face is the other alternative to the one-colour block
			s.addOneColourAlt(a, m, n, 1<<kop.C, n, n)
		case r > 0:
			s.addOneColour(a, n) // the activation's mana is all one colour of the pick
		}
	}
	if m == 0 || !ok {
		return
	}
	for c := range xc {
		nx := xp[c]
		if both && !colours {
			nx = xc[c]
		}
		if nx > 0 {
			a.addUnits(1<<c, nx)
		}
	}
	if oneType {
		return // the extras above are the pick's too: nothing is modal
	}
	if r > 0 && both && colours {
		// the modal model varies the base unit only: a pick-dependent extra
		// has no per-mode price
		s.capFull(StallTapExtra)
		return
	}
	if !both || !colours || xc == xp {
		return
	}
	for c := range xc {
		if c != kop.C && xc[c] != xp[c] {
			s.capFull(StallTapExtra)
			return
		}
	}
	if ms != nil && ms.n < len(ms.m) {
		a.remove(m, 1) // held out: the caller tries each source's mode
		ms.m[ms.n], ms.e[ms.n] = m, xc[kop.C]-xp[kop.C]
		ms.n++
		return
	}
	if !tight || pc.painN != 1 || s.g.availX&(availAny|availWin|availWinCol) != 0 || k.gen.anyContGrant {
		s.capFull(StallTapExtra)
		return
	}
	a.nModal++
}

// reflectOneType reports whether a painland's {C} ability and its coloured
// pick differ in their extras by the reflected mana r of {C} alone: the {C}
// ability's extra xc holds r more {C} than the pick's xp (a reflect carrier
// adds one mana of the type the tap produced, and a {C} tap produced {C}),
// everything else equal. The source then makes 1+r mana of one type either
// way, which addPainAvail holds as one unit with a {C} alternative.
func reflectOneType(xc, xp *[kop.NumManaKinds]uint8, r uint8) bool {
	d := *xc
	if d[kop.C] < r {
		return false
	}
	d[kop.C] -= r
	return d == *xp
}

// modalSet holds the modal sources of a walk that tries each one's mode
// itself (affordsUngrantedFor): the unit mask of its two modes and the {C}
// its {C} ability brings beyond the colour pick's extra.
type modalSet struct {
	n int
	m [3]uint8
	e [3]uint8
}

// modes returns the avail of mode choice c over the held-out sources: bit i
// set is source i making {C} and its extra, clear a colour.
func (ms *modalSet) modes(a avail, c int) avail {
	for i := 0; i < ms.n; i++ {
		if c>>i&1 != 0 {
			a.addUnits(1<<kop.C, 1+ms.e[i])
		} else {
			a.addUnits(ms.m[i], 1)
		}
	}
	return a
}

// painExtraC is the {C}-only extra a painful source's {C} ability brings
// beside its coloured one (srcExtraPain): what painTight holds in its group.
func (s *State) painExtraC(k *prep, sl Slot, pc *pcard) uint8 {
	if pc.painOnly || k.gen == nil || !k.gen.anyTapExtra {
		return 0
	}
	xc, xp, _, ok := s.srcExtraPain(k, sl, pc)
	if !ok {
		return 0
	}
	return xc[kop.C] - xp[kop.C]
}

// dynPairOne reports whether pc is an evaluated pair's wide view of any colour
// (The Beacon of Kahless's Battlefront, mana_dynpair.go): a tap makes wideN
// mana of ONE chosen colour (CR 106.1, the ability's "choose a color"), not
// wideN independent any-colour units, so the census prices it as a one-colour
// record beside the any-colour census ability, as gorge's planner counts it
// (pay planalts.go: one alternative of wideN mana per colour).
func dynPairOne(pc *pcard) bool {
	return !DebugDynPairIndependent && pc.wideN > 1 && pc.wideCol == kop.ColAny && !pc.wideChosen && pc.manaAb >= 0
}

// DebugDynPairIndependent prices an evaluated pair's wide view as independent
// any-colour units (the old behaviour); a mutation switch for the lockstep test.
var DebugDynPairIndependent bool

// tapAnyColDeferExtra reports whether a Taps-deferred gap in m would float an
// any-colour StatTapExtra when the search activates it: the extra an
// unfoldable Produced$ Any / Combo Any TapsForMana trigger adds as it resolves
// at the tap (addGatedExtra), which every priced read leaves out
// (tapExtraReaches) -- so gorge's census cannot price it (paymentPlanCensusOf:
// the trigger joins no census unit) and its one prefix search over the census
// gaps is the activation's proof (potentialCensusFallback). The kernel
// searches the gap beside the filters (otherGaps) and alone on a filterless
// board only when this holds, so a decline elsewhere stays the proof gorge's
// census makes it.
func (s *State) tapAnyColDeferExtra(k *prep, m slotMask) bool {
	if k.gen == nil || !k.gen.anyTapExtra || DebugTapAnyColOff || s.pool != nil && s.pool.census {
		return false
	}
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		ctl := s.perms[sl].ctrl
		found := false
		s.eachStatic(k, func(st statSrc) bool {
			a := &st.a.Static
			if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || st.sl == NoSlot ||
				a.StFlags&kop.StTapAnyCol == 0 || a.Cond2 != 0 ||
				st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 ||
				a.StFlags&(kop.StTapReflect|kop.StTapOnC) != 0 {
				return true
			}
			if !s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
				return true
			}
			found = true
			return false // stop at the first extra the gap's tap brings
		})
		if found {
			return true
		}
	}
	return false
}

// prefixAnyProbe is scriptPrefixAny's availX for its "does the plain walk list
// this play" probe. On a filterless board whose Taps-deferred gap brings an
// any-colour StatTapExtra, actScriptPaysCost lists a play only the gap search
// pays (tapAnyColDeferExtra). That listing is the script's own, not the plain
// census's, so it must not stop the prefix from tapping the gap. Otherwise the
// mana step's plan taps the land without the trigger's extra, while gorge's
// tap resolves it (Dawn's Reflection on the Mountain paying Shiv's Embrace's
// {R} floats the two extra mana in gorge only). This is the availNoActScript
// probe scriptPrefix uses; elsewhere the probe stays the full listing.
func (s *State) prefixAnyProbe(k *prep, p uint8) uint16 {
	if !s.scriptFilt(k, p).any() && s.tapAnyColDeferExtra(k, s.scriptTapsDefer(k, p)) {
		return availNoActScript
	}
	return 0
}

// anyColOwedRun reports the any-colour extra's second answer is live
// (genState.rsv[3], set around deferGapSearch's second run and the replay of
// the line it finds).
func (s *State) anyColOwedRun() bool { return s.g.rsv[3] != 0 }

func (s *State) setAnyColOwedRun(on bool) {
	s.g.rsv[3] = 0
	if on {
		s.g.rsv[3] = 1
	}
}
