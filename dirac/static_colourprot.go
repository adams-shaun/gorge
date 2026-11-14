package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Protection from the chosen colour until end of turn (Akroma's Blessing,
// Brave the Elements, Reverent Mantra: "Choose a color. Creatures you
// control gain protection from the chosen color until end of turn."; gorge
// effChooseColor, then effProtectionAll Gains$ ChosenColor and
// grantProtectionUntilEOT). ExtChooseColorRes asks the colour; the
// ExtColorGrant with C bit cgProt that follows marks, for every battlefield
// permanent matching its filter as it resolves (CR 611.2c: the set is fixed
// then; or the one Obj without a filter), the arena slot bit of that colour
// (mechProtW..mechProtG). protPrinted reads the marks beside the printed
// colours, so damage (protColFrom), blocking, targeting and attachment all
// see them (CR 702.16). The marks end in the cleanup step (CR 514.2) and
// when a new permanent takes the slot (arenaClrSlot, CR 400.7). A grant is
// a layer-6 effect: a loss of all abilities removes an earlier one, and a
// grant made while the permanent had lost its abilities also sets the
// post-loss twin (mechProtPost*), which holds through the loss (CR 613.7;
// the combat_mustbeblocked.go rule).
//
// res.col is the same answer kept for the resolution's ChosenColor filters
// (resColour): Wash Out's bounce, Root Greevil's destruction, Addle's
// discard pick (gorge foldChoose records it on the source, and
// chosenColorMatches reads the source's). A Discard rewrites res.chosen, so
// the filters read the copy.

// cgProt, in an ExtColorGrant's C: protection from the chosen colour.
const cgProt uint8 = 4

// cgProtAll, beside cgProt: protection from each colour (a mark for all
// five), the grant of gorge's "Protection from each color" Pump keyword
// (Akroma's Will) rather than of the resolution's chosen colour.
const cgProtAll uint8 = 8

// prepColProt arms the protection-from-a-colour reads (anyProtCol) and the
// damage sites (rbProtCol) for card i's protection grant.
func prepColProt(in *kop.Instr, g *genPrep, i int) {
	if uint8(in.C)&cgProt != 0 {
		g.anyProtCol = true
		g.replBits[i] |= rbProtCol
	}
}

// colProtUses reports the protection marks' need (arenaUses).
func colProtUses(in *kop.Instr, u *[numMech]bool) {
	if in.Ext() == kop.ExtColorGrant && uint8(in.C)&cgProt != 0 {
		for c := mech(0); c < 5; c++ {
			u[mechProtW+c], u[mechProtPostW+c] = true, true
		}
	}
}

// colProtGrant is ExtColorGrant with cgProt: Obj, or every battlefield
// permanent matching F (the sweep fixed before any mark lands), gains
// protection from res.chosen's colour.
func (s *State) colProtGrant(k *prep, in *kop.Instr) {
	first, last := mech(0), mech(0)
	if uint8(in.C)&cgProtAll != 0 {
		last = 4 // every colour
	} else {
		c := s.res.chosen
		if c == 0 || c > 5 {
			return
		}
		first, last = mech(c-1), mech(c-1)
	}
	if in.F != 0 {
		var ord [MaxPerm]Slot
		n := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:n] {
			for m := first; m <= last; m++ {
				s.colProtMark(k, sl, m)
			}
		}
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		for m := first; m <= last; m++ {
			s.colProtMark(k, sl, m)
		}
	}
}

func (s *State) colProtMark(k *prep, sl Slot, c mech) {
	if !s.mSl(k, mechProtW+c, sl) {
		s.mSlSet(k, mechProtW+c, sl)
		s.touchSl(sl)
	}
	if !s.mSl(k, mechProtPostW+c, sl) && s.genLost(k, sl) {
		s.mSlSet(k, mechProtPostW+c, sl)
		s.touchSl(sl)
	}
}

// colProtOf is the colours the permanent at sl has a granted protection
// from (protPrinted's grant half): a mark whose abilities are not lost, or
// a post-loss mark.
func (s *State) colProtOf(k *prep, sl Slot) kop.ColorMask {
	var out kop.ColorMask
	lost, lostKnown := false, false
	for c := mech(0); c < 5; c++ {
		if s.mSl(k, mechProtPostW+c, sl) {
			out |= kop.ColW << c
			continue
		}
		if !s.mSl(k, mechProtW+c, sl) {
			continue
		}
		if !lostKnown {
			lost, lostKnown = s.genLost(k, sl), true
		}
		if !lost {
			out |= kop.ColW << c
		}
	}
	return out
}

// colProtClear ends the grants (the cleanup step, CR 514.2), touching each
// permanent that held one.
func (s *State) colProtClear(k *prep) {
	var held slotMask
	for c := mech(0); c < 5; c++ {
		held = held.or(s.mSlMask(k, mechProtW+c))
		s.mSlZero(k, mechProtW+c)
		s.mSlZero(k, mechProtPostW+c)
	}
	for m := held.and(s.occ); m.any(); m = m.rest() {
		s.touchSl(m.first())
	}
}

// resColour is the colour index (0..4) a ChosenColor filter reads from
// this resolution's own ChooseColor answer (res.col): for a filter whose
// source is the resolving entry's (src the source's battlefield slot, or
// NoSlot for a spell or a source that left). ok=false: no such answer.
func (s *State) resColour(src Slot) (uint8, bool) {
	if !s.res.active || s.res.col == 0 || s.res.col > 5 {
		return 0, false
	}
	if src != NoSlot {
		if _, sl := s.resolutionTypeSource(); sl != src {
			return 0, false
		}
	}
	return s.res.col - 1, true
}

// colProtKw is the oracle projection's keyword bits for the permanent at
// sl's granted protections: gorge lists each as a "Protection from
// <colour>" keyword, and black and red have kernel bits (KwProtBlack,
// KwProtRed); a colour the card prints protection from is dropped, as the
// oracle drops gorge's printed spelling.
func (s *State) colProtKw(k *prep, sl Slot) kop.KwMask {
	g := s.colProtOf(k, sl) &^ k.t.Cards[s.cardAt(sl)].ProtColors
	var kw kop.KwMask
	if g&kop.ColB != 0 {
		kw |= kop.KwProtBlack
	}
	if g&kop.ColR != 0 {
		kw |= kop.KwProtRed
	}
	return kw
}
