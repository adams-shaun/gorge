package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A printed trigger's own ExileFromGrave<N/Spec> cost (Masked Vandal; gorge
// cumulative.go's triggered-cost window: the trigger_cost pay/decline, then
// one "exile_cost" KChoose over the payer's graveyard cards matching Spec in
// zone order, unasked with exactly N candidates): PayExileGrave.

// countGraveExile counts p's graveyard cards matching f.
//
//go:noinline
func (s *State) countGraveExile(k *prep, p uint8, f kop.FilterID) int {
	pl := &s.pl[SeatIx(p)]
	n := 0
	for i := 0; i < int(pl.graveN); i++ {
		if o := pl.grave[i]; s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
			n++
		}
	}
	return n
}

// graveExileOptions poses the pick (ChGenPayExile).
//
//go:noinline
func (s *State) graveExileOptions(k *prep, d *Decision, p uint8) {
	d.Sub = ChGenPayExile
	if s.g.pickF&pfModes != 0 {
		d.Sub = ChCostExile // an unless exile: gorge's exilecost
	}
	f := s.genPickFilter(k)
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.graveN); i++ {
		if o := pl.grave[i]; s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// DebugSylviaNoExile drops the graveyard exile of an ExileFromGrave cost (the
// sylviatilly mutation test: the games must diverge).
var DebugSylviaNoExile bool

// exileFromGrave exiles graveyard card o (a cost: no replacement reads it).
func (s *State) exileFromGrave(o Obj) {
	if DebugSylviaNoExile {
		return
	}
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneExile, false, from, false)
}
