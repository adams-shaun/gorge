package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A resolving mana colour choice (Chandra, Hope's Beacon: "Add two
// mana in any combination of colors", a loyalty ability, so never a mana
// ability, CR 605.1a; gorge effects.askManaChoice, resumed through the
// "mana_color" arm). ExtManaChoice (B the amount, constant; C the colour mask,
// bit 8 the any-one-colour form) asks the controller one colour per mana for
// a Combo (a pick per unit, ChGenManaChoice, mode = W U B R G = 0..4; gorge
// poses it as one multi-select of the amount over the colours repeated per
// unit, which the oracle sequences), or one colour for all of the amount for
// Produced$ Any. Each answer puts its mana into the pool at once; no
// priority falls between the picks.

// raManaChoice is the resolution ask of the colour pick; ChGenManaChoice the
// decision's choice kind.
const raManaChoice uint8 = 206

// ChGenManaChoice: mode = the colour (W U B R G = 0..4) of one mana a resolving
// Mana effect adds (gorge's "mana" option of the mana_color choice).
const ChGenManaChoice ChoiceKind = 206

// manaChoiceStart is ExtManaChoice; false = suspended on the first pick.
//
//go:noinline
func (s *State) manaChoiceStart(k *prep, in *kop.Instr) bool {
	n := s.value(k, in.B)
	if n <= 0 || bits.OnesCount16(uint16(in.C&0x1f)) < 2 {
		return true // kgen lowers only 1..7 mana over two or more colours
	}
	s.g.pickLeft = uint8(n)
	s.res.ask, s.res.askP = raManaChoice, s.res.e.ctrl
	return false
}

// askOptionsManaChoice builds the colour pick (the tail of askOptionsNumTimes).
//
//go:noinline
func (s *State) askOptionsManaChoice(k *prep, d *Decision) bool {
	if s.res.ask != raManaChoice {
		return s.askOptionsBreach(k, d) // act_millthenreturn.go
	}
	d.Sub = ChGenManaChoice
	c := s.prog(k)[s.res.pc].C
	for i := uint8(0); i < 5; i++ {
		if c&(1<<i) != 0 {
			d.add(MakeAction(AkChoose, 0, 0, i))
		}
	}
	return true
}

// applyAskManaChoice adds the picked mana and asks again until the amount is
// chosen (the tail of applyAskNumTimes).
//
//go:noinline
func (s *State) applyAskManaChoice(k *prep, a Action) bool {
	if s.res.ask != raManaChoice {
		return s.applyAskBreach(k, a) // act_millthenreturn.go
	}
	in := &s.prog(k)[s.res.pc]
	p := s.res.e.ctrl
	n := uint8(1)
	if in.C&0x100 != 0 {
		// Produced$ Any: one colour for the whole amount
		n = s.g.pickLeft
	}
	if in.F != 0 {
		s.manaChoiceAdd(k, p, int(a.Mode()), n, in.F) // mana_resolverestriction.go
	} else {
		s.pl[p].pool[a.Mode()] += n
	}
	s.g.pickLeft -= n
	if in.C&0x200 != 0 {
		s.persistAdd(p, int(a.Mode()), int(n)) // PersistentMana$ True (mana_persistent.go)
	}
	if s.g.pickLeft > 0 {
		return true // the next unit's pick: the ask stands
	}
	s.continueRes(k, 0)
	return true
}
