package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
// as one plain tap (a choice among colours beside {C}, a sacrificed,
// painful, restricted or evaluated-amount source, a filter's or a gate's
// tap).
const StallTapExtra = 143

func init() { stallNames[StallTapExtra] = "tapextra" }

// srcExtra is the mana the controller's StatTapExtra statics add when the
// permanent at sl taps for mana through the ability pc describes. A
// source the extra cannot be modelled on stalls (StallTapExtra) and adds
// nothing.
func (s *State) srcExtra(k *prep, sl Slot, pc *pcard) (xv [kop.NumManaKinds]uint8) {
	if pc.manaAb < 0 || pc.manaSacOnly {
		return xv // no tap: the trigger does not fire
	}
	ctl := s.perms[sl].ctrl
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || st.ctrl != ctl || st.sl == NoSlot ||
			!s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		if a.StFlags&kop.StTapOnC != 0 {
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
	if hit && (!s.tapExtraPlain(pc) || xv == [kop.NumManaKinds]uint8{}) {
		s.capFull(StallTapExtra)
		return [kop.NumManaKinds]uint8{}
	}
	return xv
}

// tapExtraPlain: the source is one plain {T} ability whose mana is n of one
// fixed colour or one colour of a choice.
func (s *State) tapExtraPlain(pc *pcard) bool {
	return pc.manaTap && !pc.manaSac && !pc.manaDyn && !pc.manaOne && !pc.manaMulti && !pc.manaChosen &&
		pc.painCol == 0 && pc.restrU == 0 && pc.rankCol == 0
}

// tapExtraStall stalls when sl's tap would have brought extra mana through a
// path that does not add it (a filter's, a gate's, a sacrificed source's).
func (s *State) tapExtraStall(k *prep, sl Slot) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	pc := s.manaPCAt(k, sl)
	if x := s.srcExtra(k, sl, pc); x != [kop.NumManaKinds]uint8{} {
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
	a.remove(s.srcMaskAt(pc, sl), pc.manaN)
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
