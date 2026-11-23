package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// riderPinCount is the pips gorge's planManaUnderRider (rules
// payment_plan.go) keeps coloured when it re-plans a may-play cast under the
// any-colour rider (pay.PlanManaUnderRiderAlternatives): every plain
// coloured pip plus every two-colour hybrid one. 0 for a shape the pins do
// not read (an X cost, a twobrid cost: its Hyb byte is the marker).
func riderPinCount(raw kop.ManaCost) int {
	if raw.X != 0 || raw.Hyb[0] == kop.HybTwobrid {
		return 0
	}
	n := 0
	for c := 0; c < kop.C; c++ {
		n += int(raw.Pips[c])
	}
	for _, h := range raw.Hyb {
		if h != 0 {
			n++
		}
	}
	return n
}

// foldColouredUnits plans folded (the cost with its rider-relaxed pips
// generic) without tapping and counts the plan's units that can make a
// coloured mana: "any color" never includes colourless (CR 107.4c), so only
// those pay a relaxed pip. found is false when the pool already pays folded
// or no plan exists.
func (s *State) foldColouredUnits(k *prep, p uint8, exclude Obj, folded kop.ManaCost) (col int, found bool) {
	pk := planner{dry: true}
	s.tapForPK(k, p, &folded, 0, exclude, false, false, &pk)
	if !pk.found {
		return 0, false
	}
	for i := 0; i < pk.bestN; i++ {
		if ty := &pk.types[pk.bestTy[i]]; ty.mask&0x1f != 0 {
			col += int(ty.n)
		}
	}
	return col, true
}

// tapRiderPinned is planManaUnderRider's re-plan for a may-play cast under
// the any-colour rider (CR 609.4b): gorge plans the folded cost first and
// keeps that plan only while its units can pay every relaxed pip coloured
// (planPaysManaUnderRider). A fold that took a colourless-only ability (a
// painland's {C}) beside a coloured source is dropped, and the pins are
// planned in turn, one per colour in WUBRG order (the cost with its nPin
// pips held coloured on that colour, pay.PlanManaUnderRiderPinned), then the
// board spread (pay.PlanManaUnderRiderSpread): the first with a plan is
// tapped, so a painland's coloured pain tap pays when no earlier colour has
// a source. It reports whether a pin was tapped; false leaves the folded
// plan to the caller (the fold proves, or no pin has a plan and gorge's
// planner offers no payment at all).
func (s *State) tapRiderPinned(k *prep, e stackEntry, folded kop.ManaCost, nPin int, av *avail) bool {
	p := e.ctrl
	col, found := s.foldColouredUnits(k, p, e.obj, folded)
	if !found {
		return false
	}
	for c := 0; c < kop.C; c++ {
		col += int(s.pl[p].pool[c])
	}
	if col >= nPin || int(folded.Generic) < nPin {
		return false
	}
	for c := 0; c < kop.C; c++ {
		pin := folded
		pin.Generic -= uint8(nPin)
		pin.Pips[c] = uint8(nPin)
		if s.tapPinTry(k, e, &pin) {
			return true
		}
	}
	if pin, ok := av.spreadPin(folded, nPin); ok {
		return s.tapPinTry(k, e, &pin)
	}
	return false
}

// tapPinTry plans pin strictly (its coloured pips are that colour's units)
// and taps the plan when one exists; a failed trial taps nothing.
func (s *State) tapPinTry(k *prep, e stackEntry, pin *kop.ManaCost) bool {
	pk := planner{try: true}
	s.tapForPK(k, e.ctrl, pin, int(e.x), e.obj, false, false, &pk)
	return pk.found
}
