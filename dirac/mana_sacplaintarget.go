package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// plainSacKeepsTarget: gorge's potential walk refuses a plain cast whose
// witness payment sacrifices every permanent the spell could target (the
// witness lowers the plan before the cast, potentialWitnessReaches), unless
// the target is announced before the mana is paid and some candidate survives
// the plan (witnessAnnouncesBeforePaying, CR 601.2c-h): Double Cleave over the
// one creature, a Catalyst Elemental that sacrifices itself for {R}{R}, is
// "unsupported witness_unreachable". It is the plain-cast form of kickWitness
// and xWitnessEats's target read (a spell with a second target slot is not
// read, announcedTargetKept).
func (s *State) plainSacKeepsTarget(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	if !k.sacMana || cost.X != 0 || !s.anySacSource(k, p) {
		return true
	}
	ab := spellAb(k, c)
	if ab == 255 || tspec(k, c, ab, 0) == nil {
		return true
	}
	tmp := s.fork()
	defer s.unfork()
	n0 := tmp.npend
	var pk planner
	pk.try = true
	tmp.tapForPK(k, p, cost, 0, o, false, false, &pk)
	if !pk.found || !plannerConsumesSac(&pk) {
		return true
	}
	return tmp.spellTargetsFeasible(k, c, p) || s.announcedTargetKept(k, tmp, c, p) || tmp.witnessEndsGameWon(k, p, c, n0)
}
