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
	case s.g.pre.state == 1 || s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice:
		c = s.objs[s.g.pre.obj].card
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

// chosenNumberOf is ExChosenNumber: the number the permanent at sl chose as
// it entered; 0 when it chose none (its chosen byte is 255) or sl holds no
// permanent, gorge's zero ChosenNumber.
func (s *State) chosenNumberOf(sl Slot) int {
	if sl == NoSlot || !s.occ.has(sl) || s.perms[sl].chosen == 255 {
		return 0
	}
	return int(s.perms[sl].chosen)
}
