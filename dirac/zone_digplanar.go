package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// zone_digplanar.go is execDigUntil's planar arm (W == kop.RefPlanarDeck, DigZone$
// PlanarDeck): the scan digs the seat's planar deck's face-down remainder and
// both placements -- found (FoundDestination$) and revealed rest
// (RevealedDestination$) -- land on that deck's bottom, mirroring gorge
// diguntil.go's planar arm.
//
// The scan starts BEHIND the face-up current plane at the record's head:
// Forge's planar-deck zone holds exactly the non-current planes, and CR
// 901's current plane is already public, so "reveal cards from the top of
// your planar deck" starts behind it (gorge's arm scans zone[1:]). Every
// card in the record is scanned in order until A cards match Valid$ (Amount$
// 1 for the corpus carrier); the found cards are remembered (DuRememberFound
// replaces the resolution's remembered set) and then moved -- found cards in
// found order, revealed rest in reveal order -- each a remove-and-append that
// lands the card at the record's tail, face up (only the head is revealed and
// it never moved; gorge's same-zone MoveZone fold appends at the zone list's
// tail and clears the moved card's FaceDown, so a dug plane sits at the
// deck's bottom face up until the next shuffle or planeswalk -- the mask
// planeDeckToBottom keeps). The deck's top-first order survives the dig.
//
// The dig parks nothing and asks nothing (kgen refuses the ask-carrying
// riders beside the planar scan), so there is no resume arm. The public
// reveal is a Note in gorge's log, which folds no state and has no kernel
// counterpart (events.Note's only fold is the coin-flip counter).

// digUntilPlanar is OpDigUntil's planar arm (W == kop.RefPlanarDeck): one scan
// of each scanned
// player's planar record (the resolving controller for the corpus carrier,
// whose DigUntil names no Defined$). Returns done=true: the dig never parks.
func (s *State) digUntilPlanar(k *prep, in *kop.Instr) bool {
	q := s.eachPlayers(k, in.Obj)
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		ps := s.planeCold()
		if ps == nil {
			continue
		}
		n := int(ps.n[p])
		if n < 2 {
			continue // the face-down remainder is empty: nothing to reveal
		}
		want := s.value(k, in.A)
		if want < 1 {
			continue // a zero Amount$ reveals nothing (gorge's amount > 0 gate)
		}
		var fnd, rev [MaxPlane]Obj
		nf, nr := 0, 0
		for j := 1; j < n; j++ {
			o := ps.deck[p][j]
			if nf < len(fnd) && nf < want && s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
				fnd[nf] = o
				nf++
				if nf >= want {
					break
				}
				continue
			}
			rev[nr] = o
			nr++
		}
		// The found cards move first (gorge's found-move block, in found
		// order), then the revealed rest in reveal order (gorge
		// digUntilReturnRest): each a remove-and-append at the record's tail.
		for _, o := range fnd[:nf] {
			s.planeDeckToBottom(ps, p, o)
		}
		for _, o := range rev[:nr] {
			s.planeDeckToBottom(ps, p, o)
		}
		if in.C>>3&kop.DuRememberFound != 0 {
			s.digUntilRemember(i == 0, fnd[:nf])
		}
	}
	return true
}

// planeDeckToBottom moves card o to seat p's planar record's tail (the deck's
// bottom), the fold of gorge's same-zone MoveZone: the zone append, and the
// moved card's FaceDown cleared (gorge's Move fold resets the field on the
// move, so a dug plane sits at the bottom face up -- planeSeat.up). A card
// that is not a member stays put.
func (s *State) planeDeckToBottom(ps *planeSeat, p uint8, o Obj) {
	d := ps.deck[p][:]
	n := int(ps.n[p])
	for i := 0; i < n; i++ {
		if d[i] != o {
			continue
		}
		copy(d[i:n-1], d[i+1:n])
		d[n-1] = o
		// the face-up mask follows the members: cards above i shift down a
		// slot, the moved card lands face up at the tail
		up := ps.up[p]
		lo := up & (1<<uint(i) - 1)
		hi := up >> (i + 1)
		ps.up[p] = lo | hi<<(i) | 1<<(n-1)
		return
	}
}
