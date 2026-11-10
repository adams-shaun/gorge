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
// extra of its normal ability, xp the extra of the painful one, which only a
// carrier that is not limited to a {C} tap reaches (gorge addTapExtra adds
// per alternative). False, with the stall counted, when either depends on the
// pick or the source is not one plain tap.
func (s *State) srcExtraPain(k *prep, sl Slot, pc *pcard) (xc, xp [kop.NumManaKinds]uint8, ok bool) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return xc, xp, true
	}
	if !s.tapExtraPlainBase(pc) {
		if _, hit := s.srcExtraHit(k, sl, pc); hit {
			s.capFull(StallTapExtra)
			return xc, xp, false
		}
		return xc, xp, true
	}
	if !pc.painOnly {
		var hit bool
		if xc, hit = s.srcExtraHit(k, sl, pc); hit && xc == [kop.NumManaKinds]uint8{} {
			s.capFull(StallTapExtra)
			return xc, xp, false
		}
	}
	xp = s.srcExtraColour(k, sl, pc)
	return xc, xp, true
}

// srcExtraHit is srcExtra without the stall: the extra vector, and whether any
// StatTapExtra of the controller reaches a tap of sl for mana (the vector is
// empty when the extra depends on the pick).
func (s *State) srcExtraHit(k *prep, sl Slot, pc *pcard) (xv [kop.NumManaKinds]uint8, hit bool) {
	return s.srcExtraAlt(k, sl, pc, false)
}

// srcExtraColour is the extra a coloured pick of sl's painful alternative
// brings: the carriers not limited to a {C} tap.
func (s *State) srcExtraColour(k *prep, sl Slot, pc *pcard) (xv [kop.NumManaKinds]uint8) {
	xv, _ = s.srcExtraAlt(k, sl, pc, true)
	return xv
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
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if !s.tapExtraReaches(k, st, sl, ctl) {
			return true
		}
		if a.StFlags&kop.StTapReflect != 0 {
			// one mana of the type the tap produced: fixed for a source of one
			// type, the pick's otherwise (and a filter's colour is picked after
			// its tap); a choice source's folded view already counts it
			// (mana_taprefl.go)
			if pc.reflFold {
				return true
			}
			m := srcMask(pc)
			if colour || m == 0 || m&(m-1) != 0 {
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
// costs no {T} (a once-each-turn one) taps nothing and brings none. The colour
// is picked after the tap, so a carrier limited to a {C} tap does not reach it.
func (s *State) filtExtra(k *prep, sl Slot) (xv [kop.NumManaKinds]uint8) {
	if k.gen == nil || !k.gen.anyTapExtra || s.filtAb(k, sl).Cost.Flags&kop.CostTap == 0 {
		return xv
	}
	xv, _ = s.srcExtraTapped(k, sl, s.manaPCAt(k, sl), true)
	return xv
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
			if pc.reflFold {
				return true // the folded view's own mana (mana_taprefl.go)
			}
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

// tapExtraReaches reports whether st is a StatTapExtra static that adds its
// mana when the permanent at sl, controlled by ctl, taps for mana: its
// controller's own taps, or every player's under StTapAnyPlayer (the extra
// then goes to the tapping player, whose pool the caller fills).
func (s *State) tapExtraReaches(k *prep, st statSrc, sl Slot, ctl uint8) bool {
	a := &st.a.Static
	if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || st.sl == NoSlot ||
		st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 {
		return false
	}
	return s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull)
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
	if x := s.srcExtra(k, sl, pc); x != pc.manaXv {
		s.capFull(StallTapExtra)
	}
}

// addExtra floats the extra mana of sl's tap in p's pool.
func (s *State) addExtra(k *prep, p uint8, sl Slot, pc *pcard) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xv := s.srcExtra(k, sl, pc)
	for c := range xv {
		s.pl[p].pool[c] += xv[c]
	}
}

// addExtraCol is addExtra for a tap that made colour col: a painful source's
// coloured pick brings only the extra of the carriers not limited to a {C}
// tap, its {C} ability all of them (srcExtraPain).
func (s *State) addExtraCol(k *prep, p uint8, sl Slot, pc *pcard, col int) {
	if pc.painCol == 0 {
		s.addExtra(k, p, sl, pc)
		return
	}
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xc, xp, ok := s.srcExtraPain(k, sl, pc)
	if !ok {
		return
	}
	if pc.painOnly || uint8(pc.manaCol)&(1<<col) == 0 {
		xc = xp
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
	if k.gen != nil && s.g.noGrant && s.g.availX&availPlain != 0 && s.manaGranted(k, sl) {
		return // an activated ability's prefix search leaves a granted-only source out of a (availFor): nothing of it to take out
	}
	if k.gen != nil && k.gen.anyContGrant {
		var gi [maxGrants]int16
		if s.grantIdxs(k, sl, &gi) > 0 {
			// a source with a granted mana ability is one unit of its own
			// and the granted colours together (availGrant): taking out its
			// own mask alone would leave that unit standing, and the tap
			// that pays the cost would fund the cost's mana too
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
	a.remove(s.srcMaskAt(pc, sl), srcYield(pc))
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xv := s.srcExtra(k, sl, pc)
	for c := range xv {
		if xv[c] > 0 {
			a.remove(1<<c, xv[c])
		}
	}
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
	if m != 0 {
		n := uint8(1)
		if pc.painOnly {
			n = max(pc.manaN, 1)
		}
		a.remove(m, n)
	}
	if m == 0 || k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xc, xp, ok := s.srcExtraPain(k, sl, pc)
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

// addPainAvail adds a painful source's units to a: its production mask (the
// offer gate's, painAvailMask) and the extra its tap brings. The extra of a
// coloured pick is the carriers not limited to a {C} tap; the {C} ability
// gets all of them. A source offering both makes either {C} and the {C}-only
// extra, or a colour: units cannot say that, so with a {C}-only extra it is
// counted as a modal source (avail.nModal) that painTight splits by group,
// where the board allows it (modal), else the table stalls.
func (s *State) addPainAvail(k *prep, sl Slot, pc *pcard, p uint8, a *avail, ms *modalSet, tight bool) {
	m := s.painAvailMask(pc, p)
	if m != 0 {
		n := uint8(1)
		if pc.painOnly {
			n = max(pc.manaN, 1) // a pain-only source's whole activation (Ancient Tomb's {C}{C})
		}
		a.addUnits(m, n)
	}
	if m == 0 || k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xc, xp, ok := s.srcExtraPain(k, sl, pc)
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
			a.addUnits(1<<c, n)
		}
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
	xc, xp, ok := s.srcExtraPain(k, sl, pc)
	if !ok {
		return 0
	}
	return xc[kop.C] - xp[kop.C]
}
