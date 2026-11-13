package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Playing a land-front Adventure from exile (CR 715.3d, gorge),
// cold: reached only when the table has a card with faces
// (genPrep.anyFaces). A land main face is never cast (CR 305.1), so the
// recast walk withholds it; "play it" is instead the ordinary land play,
// which shares the turn's land drop and moves the card from exile
// (playLandNow, detaching clears ofAdventure).

// offerAdventureLandPlays offers playing each of p's land-front Adventure
// cards resting on an adventure (ofAdventure) from exile: gorge's
// exileCastsWalk play_land arm (rules legal_walk_alt.go, CR 715.3d). It is
// offered at sorcery timing only while a land drop remains, exactly like
// the hand's land play. A may-play grant over the same exile card may have
// offered the play already (priorityGen runs first): never add a second
// option for one object (gorge hasPlayLandOption).
func (s *State) offerAdventureLandPlays(k *prep, d *Decision, p uint8, sorc bool) {
	if !sorc {
		return
	}
	pl := &s.pl[p]
	if pl.landsPlayed >= s.landLimit(k, p) {
		return
	}
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		if s.objs[o].of&ofAdventure == 0 {
			continue
		}
		c := s.objs[o].card
		if !k.cards[c].land || k.t.Cards[c].FaceMode != kop.FaceAdventure {
			continue
		}
		if playLandOffered(d, o) {
			continue
		}
		d.add(MakeAction(AkPlayLand, TCode(o), 0, 0))
	}
}

// playLandOffered reports whether d already lists a land play for object o
// (gorge hasPlayLandOption: one option per land, whichever grant offered
// it).
func playLandOffered(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkPlayLand && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}
