package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The blind face-down exile of a ChangeZone or ChangeZoneAll (Induced
// Amnesia's hand, Triumph of Saint Katherine's own card): ExileFaceDown$ True
// with no WithMayLook$ and no DefinedExiler$. gorge's mover calls
// applyMoveFaceDown (engine/effects/zone_change.go), which stamps the blind
// exiled_with_face_down marker; the fold (engine/events/apply_zone.go) sets
// Object.FaceDown and ExiledWith the exiling source, and nobody is the looker.
// kgen lowers it only beside RememberChanged$ True with the move first in its
// program, so the resolution's remembered set right after the move is exactly
// the cards it moved; ExtExileFaceDown then gives each one the Dig face-down
// exile's mark (zone_exilefacedown.go faceDownExile), the looker bit read by
// no decision.

// exileFaceDownExt is ExtExileFaceDown.
//
//kmanifest:ext ExtExileFaceDown
func (s *State) exileFaceDownExt(_ *prep, _ *kop.Instr) (ok, done bool) {
	src, ctrl := s.res.e.obj, s.res.e.ctrl
	for _, o := range s.g.rem[:s.g.nrem] {
		s.faceDownExile(o, src, ctrl)
	}
	return true, true
}
