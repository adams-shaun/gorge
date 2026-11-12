package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters number choice (CR 614.12): an enters program opening with
// OpExt ExtChooseNumber (B the lowest number, C the highest; kgen
// lower_choosenumber.go). gorge asks it at the entry boundary as a KChoose of
// "number" options, the value on Amount (effects.ETBNumberChoices); the
// kernel asks it as the colour choice is (akNumber: preAskFor on the stack,
// queueAskAt mid-resolution) and keeps the answer in the permanent's chosen
// byte, which ExChosenNumber reads (Nyota Uhura, Communications' cmcEQChosen).

// numberOptions fills the as-enters number ask: one option per number from
// the program's lowest to its highest, mode = the number, ascending as
// gorge lists them.
//
//go:noinline
func (s *State) numberOptions(k *prep, d *Decision) {
	d.Sub = ChGenNumber
	lo, hi := s.numberBounds(k)
	for n := lo; n <= hi; n++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(n)))
	}
}

// numberBounds is the ExtChooseNumber bounds of the entering card whose
// number ask is being posed: the pre-entry ask's object, else the queued
// ask's permanent (colorExclude's lookup).
func (s *State) numberBounds(k *prep) (lo, hi int) {
	c := NoCard
	switch {
	case s.preAskPosed() || s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice:
		c = s.preAskCard()
	case s.g.nask > 0:
		c = s.cardAt(s.g.ask[0].sl)
	}
	if c == NoCard {
		return 0, -1
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		for j := range a.Prog {
			if in := &a.Prog[j]; in.Ext() == kop.ExtChooseNumber {
				return int(in.B.N), int(in.C)
			}
		}
	}
	return 0, -1
}

// resChosenNumber marks a mid-resolution ChooseNumber answer in res.bad
// (bits 0..MaxTargets-1 are target-illegality flags; 0x40 and 0x80 are the
// chosen type and card-type flags, so 0x20 is the free bit).
const resChosenNumber uint8 = 0x20

// numberOfferCeiling is gorge effects.chooseNumberMaxOffer: the inclusive
// upper bound of the default list when the card states no Max$.
const numberOfferCeiling = 12

// chosenNumberOf is ExChosenNumber: the number the source chose, read the way
// gorge's Object.ChosenNumber is. A permanent on the battlefield keeps it in
// its chosen byte; a resolving spell keeps it on the card object; a source
// that has left the battlefield, or a resolution whose object is already
// gone, reads the resolution's own recorded answer (the resChosenNumber
// flag). A source that chose none reads gorge's zero.
func (s *State) chosenNumberOf(sl Slot) int { return s.chosenNumberOfFor(NoObj, sl) }

// chosenNumberOfFor is chosenNumberOf with the source object, for the value
// head's spell-source read.
func (s *State) chosenNumberOfFor(src Obj, sl Slot) int {
	if sl != NoSlot {
		if s.occ.has(sl) && s.perms[sl].chosen != 255 {
			return int(s.perms[sl].chosen)
		}
	} else if src != NoObj && s.objs[src].chosen != 255 {
		return int(s.objs[src].chosen)
	}
	if s.res.active && s.res.bad&resChosenNumber != 0 {
		return int(s.res.chosen)
	}
	return 0
}

// storeChosenNumber records a mid-resolution ChooseNumber answer: on the
// source permanent's chosen byte and its card object (so a source that later
// leaves keeps it, as gorge's Object.ChosenNumber does), and in the
// resolution for an answer read after the slot or object is gone.
func (s *State) storeChosenNumber(o Obj, sl Slot, n int) {
	s.res.chosen = uint8(n)
	s.res.bad |= resChosenNumber
	if sl != NoSlot {
		s.perms[sl].chosen = uint8(n)
	}
	if o != NoObj {
		s.objs[o].chosen = uint8(n)
		s.touch(o)
	}
}

// chooseNumberMid runs ExtChooseNumberMid. B is the Max$ value (read when
// C&0x100 is set), C&0xff the literal Min$ floor, C&0x200 Random$ True. The
// offer is gorge chooseNumberAsk's: Min..Max, or Min..12 without a Max$; a
// bound the context cannot honour (a negative or inverted Max$) falls back to
// the deterministic 0, as gorge's fail-closed arm does. A single candidate is
// recorded unasked (gorge asks only when the list has more than one option).
// The answer is stored on the source and read by ExChosenNumber.
func (s *State) chooseNumberMid(k *prep, in *kop.Instr) bool {
	lo := int(in.C & 0xff)
	hasMax := in.C&0x100 != 0
	random := in.C&0x200 != 0
	hi := numberOfferCeiling
	if hasMax {
		hi = s.value(k, in.B)
		if hi < 0 || lo > hi || hi > 1000 {
			lo, hi = 0, 0 // gorge's loud fail-closed fallback: the deterministic 0
		}
	} else if lo > numberOfferCeiling {
		lo = 0 // gorge's Min$-only fallback keeps the default 0..12 list
	}
	if hi > 255 {
		// The decision carries the number in a byte; a bound past 255 is a
		// declared capacity limit (gorge's own list ceiling is 1000).
		hi = 255
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	o, sl := s.resolutionTypeSource()
	if random {
		n := lo
		if hi > lo {
			n = lo + s.rng.IntN(hi-lo+1)
		}
		s.storeChosenNumber(o, sl, n)
		return true
	}
	if hi > lo {
		s.res.ask, s.res.askP = raChooseNumber, p
		return false
	}
	s.storeChosenNumber(o, sl, lo)
	return true
}
