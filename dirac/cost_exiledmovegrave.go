package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A printed trigger's own ExiledMoveToGrave<N/Spec> cost (the Eldrazi
// processors: Blight Herder, Mind Raker, Ulamog's Nullifier's "you may put
// a card an opponent owns from exile into that player's graveyard. If you
// do, ..."; CR 118.3, 603.4). gorge cumulative.go's triggered-cost window:
// the trigger_cost pay/decline, then one "graveyard_cost" pick over every
// alive seat's exile zone, seats in order and each zone in list order
// (pay.MoveToGraveCandidates), unasked with exactly N candidates; each
// chosen card moves to its owner's graveyard. It rides PayExileGrave with
// B.N kop.PayBSubMoveGrave (Instr.C's nibble is full) and execPay widens it
// to payKindMoveGrave. Only the source-free filters lower (Card.OppOwn):
// kgen refuses Card.ExiledWithSource by name.

// payKindMoveGrave is the pay kind (execPay's kind, pickZ) of a trigger's
// own ExiledMoveToGrave<N/Spec> cost.
const payKindMoveGrave = 26

// moveGraveCand: exiled card o, in seat q's exile zone, can pay p's cost
// filtered by f. gorge scans every alive seat (an exiled card sits in its
// owner's zone), seats from 0, each zone in list order.
func (s *State) moveGraveCand(k *prep, p uint8, f kop.FilterID, q int, o Obj) bool {
	if s.out(uint8(q)) {
		return false
	}
	return s.matchCardGen(k, f, s.objs[o].card, uint8(q), p, o)
}

// countMoveGrave counts the exiled cards that can pay p's cost filtered by f.
//
//go:noinline
func (s *State) countMoveGrave(k *prep, p uint8, f kop.FilterID) int {
	n := 0
	for q := 0; q < int(s.nSeats()); q++ {
		pl := &s.pl[q]
		for i := 0; i < int(pl.exileN); i++ {
			if s.moveGraveCand(k, p, f, q, pl.exile[i]) {
				n++
			}
		}
	}
	return n
}

// moveGraveOptions poses the pick (ChGenPayMoveGrave).
//
//go:noinline
func (s *State) moveGraveOptions(k *prep, d *Decision, p uint8) {
	d.Sub = ChGenPayMoveGrave
	f := s.genPickFilter(k)
	for q := 0; q < int(s.nSeats()); q++ {
		pl := &s.pl[q]
		for i := 0; i < int(pl.exileN); i++ {
			if o := pl.exile[i]; s.moveGraveCand(k, p, f, q, o) {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
}

// moveGraveCost moves exiled card o to its owner's graveyard (a cost: gorge
// emits a plain MoveZone from exile, its owner's graveyard the destination).
func (s *State) moveGraveCost(k *prep, o Obj) {
	if s.objs[o].zone != kop.ZoneExile {
		return
	}
	from := s.detach(o)
	s.placeFromK(k, o, kop.ZoneGraveyard, false, from, false, false)
}
