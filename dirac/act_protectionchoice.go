package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The protection quality choice (the gap4-05 ticket's Choices$ lists; gorge
// combatfx.go askProtectionChoice): a Protection Gains$ Choice whose
// Choices$ goes beyond plain AnyColor — colours, colorless, a card type or
// the card types themselves (Giver of Runes' "AnyColor,colorless",
// Apostle's Blessing's "AnyColor,Artifact", Pippin, Guard of the Citadel's
// "CardType") — poses the controller's ask (CR 702.16), and the one of the
// card's consecutive OvProtChoice override records that names the answered
// quality grants its protection from that quality until end of turn. The
// plain AnyColor choice is not here: gorge poses it over the five colours
// (canon "color"), which is the existing ChooseColorRes + ColorGrant pair
// (lower_protection.go).
//
// kgen emits the ExtProtectionChoice instruction with B = the option count,
// then the OpOverride with B = kop.OvProtChoice naming the FIRST of the
// family's records (one per option, in gorge's offer order, differing only
// in ProtFrom: a colour mask, the no-colour clause, or a card type). The
// ask's options are derived from that family, so the oracle sees the same
// option list gorge's offers build.

// resProtChoice marks res.chosen as this resolution's protection choice
// (ExtProtectionChoice's answer, an option index). res.bad's bits
// 0..MaxTargets-1 are the target-illegality flags and resChosenType and
// resChosenCard hold the two high ones; resProtChoice is bit 5, which
// resChosenNumber (repl_choosenumber.go) shares, with res.chosen: the two
// answers alias, so kgen refuses a resolution that carries both asks
// (lower_protection.go, lower_choosenumber.go).
const resProtChoice uint8 = 0x20

// protectionChoiceExt is ExtProtectionChoice: pause for the ask.
//
//kmanifest:ext ExtProtectionChoice
func (s *State) protectionChoiceExt(k *prep, in *kop.Instr) (ok, done bool) {
	s.res.ask, s.res.askP = raProtectionChoice, s.res.e.ctrl
	return false, true
}

// protChoiceFamily locates the pending protection choice's override family:
// the ExtProtectionChoice instruction the resolution is parked at (its B is
// the option count) and the OpOverride with B = kop.OvProtChoice that
// follows it (its C is the family's first record). ok=false: the resolution
// is not parked on the ask, or the program carries no family after it.
func (s *State) protChoiceFamily(k *prep) (n, first int, ok bool) {
	if !s.res.active {
		return 0, 0, false
	}
	prog := s.prog(k)
	pc := int(s.res.pc)
	if pc < 0 || pc >= len(prog) {
		return 0, 0, false
	}
	in := &prog[pc]
	if in.Op != kop.OpExt || in.A.N != int16(kop.ExtProtectionChoice) {
		return 0, 0, false
	}
	n = int(in.B.N)
	for i := pc + 1; i < len(prog) && i < pc+32; i++ {
		if prog[i].Op == kop.OpOverride && prog[i].B.N == kop.OvProtChoice {
			return n, int(prog[i].C), true
		}
	}
	return 0, 0, false
}

// protectionChoiceOptions poses the ask: one option per quality of the
// family, in gorge's offer order (the mode is the option's position).
//
//kmanifest:askopts raProtectionChoice
func (s *State) protectionChoiceOptions(k *prep, d *Decision) {
	d.Sub = ChGenProtChoice
	n, _, ok := s.protChoiceFamily(k)
	if !ok {
		// unreachable: kgen emits the ask and its override family together
		n = 1
	}
	for j := 0; j < n; j++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(j)))
	}
}

// protectionChoiceAnswer records the answered option's position and moves
// on. An answer past the family takes the first, gorge's stand-in.
//
//kmanifest:askapply raProtectionChoice
func (s *State) protectionChoiceAnswer(k *prep, a Action) {
	n, _, ok := s.protChoiceFamily(k)
	j := int(a.Mode())
	if !ok || j >= n {
		j = 0
	}
	s.res.bad |= resProtChoice
	s.res.chosen = objWord(j)
	s.res.ask = raNone
	s.continueRes(k, 0)
}

// ovProtChoice is OpOverride's protection-choice arm (interp_gen.go): the
// hot path keeps just the one B test. The record of the family at C that
// names the answered option applies; a resolution whose ask is unanswered
// (or whose answer named no record of this family — an ask on another
// resolution) applies nothing, gorge's fail-closed case.
//
//go:noinline
func (s *State) ovProtChoice(k *prep, sl Slot, C int, card CardID, fl uint8) {
	if sl == NoSlot || s.res.bad&resProtChoice == 0 {
		return
	}
	c := int(s.res.chosen)
	if C+c >= len(k.t.Overrides) {
		return
	}
	s.applyOverrideFl(k, sl, C+c, card, fl)
}
