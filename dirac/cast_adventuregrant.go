package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An Adventure-only play permission (Mosswood Dreadknight's "When it
// dies, you may cast it from your graveyard as an Adventure until the end of
// your next turn": Effect MayPlay, Affected$ Card.IsRemembered+Adventure; gorge
// cr-mayplay-adventure-from-graveyard, mayPlayAdventureGrant /
// appendAdventureAlt). The record (kop.MayPlayAdvKind) permits the
// Adventure face's cast only (CR 715.3a), never the main face's, gated on the
// Adventure face's own timing: a face cast (castFace, target = face 1)
// through the permission (castMayPlay), spending the record.

// offerAdventureGrant offers casting r's card as its Adventure.
func (s *State) offerAdventureGrant(k *prep, d *Decision, p uint8, r mayPlay, sorc bool) {
	o := r.obj
	cd := &k.t.Cards[s.objs[o].card]
	if cd.FaceMode != kop.FaceAdventure || cd.Face != 0 || len(cd.Faces) != 2 {
		return
	}
	if s.faceCastable(k, p, o, cd.Faces[1], sorc) {
		d.add(MakeAction(AkCast, TCode(o), 1, castFace|castMayPlay))
	}
}
