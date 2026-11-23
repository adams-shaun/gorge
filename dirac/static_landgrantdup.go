package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A granted basic land type whose colour a Combo or Any ability of the land
// already makes (Foul Orchard's "{T}: Add {B} or {G}" under Urborg's Swamp).
// gorge's intrinsic-append dedup (ManaAbilitiesProduce) matches an ability
// whose Produced$ is exactly the one colour, so a Combo does not cover it and
// the land still gets the intrinsic "{T}: Add {B}" (CR 305.6, 605.1a). The
// planner ranks that intrinsic alternative above the printed one
// (pay.CompareSteps: "intrinsic" before "printed"), so a generic cost paid
// from the land floats the granted colour where the printed Produces
// tie-break would take {G}. blgView folds the colour away (the census is the
// same), so the plan unit names it as its first colour instead.

// blgSubsumedFirst is 1 + the colour a plan unit at sl makes first for a
// granted type whose colour pc's own several-colour ability already makes, in
// the order the granted colours of a nonbasic rank (pickUnitColour), 0 when
// there is none.
func (s *State) blgSubsumedFirst(k *prep, sl Slot, pc *pcard, mask uint8) uint8 {
	if k.gen == nil || !k.gen.blgOn || pc.manaAb < 0 || pc.manaIntrinsic {
		return 0
	}
	own := uint8(ownMadeCols(pc)) & 0x1f
	if own&(own-1) == 0 {
		return 0 // no colour, or one: the exact-Produced$ dedup holds
	}
	m := uint8(s.blgMask(k, sl)) & s.blgPrintedMask(k, sl) & own & mask
	for _, c := range [5]int{kop.G, kop.R, kop.B, kop.U, kop.W} {
		if m&(1<<c) != 0 {
			return uint8(c) + 1
		}
	}
	return 0
}
