package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// drainMana is ExtDrainMana: the player in Obj loses all unspent mana
// (gorge effDrainMana, events.ManaClear on each defined seat). The ordinary
// pool, its restricted batches, the artifact/treasure provenance and the
// persistent tamans all empty: Forge's clearPool(false) is not the step
// boundary's keep-persistent empty. No decision.
func (s *State) drainMana(k *prep, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p == 255 || int(p) >= len(s.pl) {
		return
	}
	s.pl[p].pool = [kop.NumManaKinds]uint8{}
	s.rbClear(p)
	s.pm[p] = [kop.NumManaKinds]uint8{}
	s.pmOn &^= 1 << p
	s.fbR[p] = 0
	s.g.poolArt[p] = 0
	s.g.defSrc[p] = 0
	s.g.poolTre[p] = [kop.NumManaKinds]uint8{}
}
