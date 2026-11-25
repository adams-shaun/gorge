package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// greatestMVArtifact is kop.W7GreatestMVArtifact on the battlefield
// permanent at sl (gorge filter.go's greatestCMC_<prop> with prop Artifact):
// sl is an artifact and no other battlefield artifact has a greater mana
// value; ties match. The mana value is gorge objectManaValue's on the
// battlefield: 0 face down (CR 708.2), else mvPerm (an embalm copy's or a
// back-face copy's 0). Cold: only the word's filters read it.
//
//go:noinline
func (s *State) greatestMVArtifact(k *prep, sl Slot) bool {
	if s.typesAt(k, sl)&kop.TArtifact == 0 {
		return false
	}
	mine := s.permMVLive(k, sl)
	for m := s.occ; m.any(); m = m.rest() {
		o := m.first()
		if o != sl && s.typesAt(k, o)&kop.TArtifact != 0 && s.permMVLive(k, o) > mine {
			return false
		}
	}
	return true
}

// permMVLive is the battlefield permanent at sl's mana value: 0 face down
// (CR 708.2), else mvPerm.
func (s *State) permMVLive(k *prep, sl Slot) int {
	if s.faceDown(k, sl) {
		return 0
	}
	return s.mvPerm(k, sl)
}
