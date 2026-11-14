package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// exileCands counts (and, with d, offers) the candidates of an exile cost
// part c: with kop.CostExileCtrlOrGrave, the payer's other permanents
// matching c.Filter (battlefield order) and then their graveyard's cards
// (gorge exAsk over exilePartZones' battlefield, graveyard order; Craft's
// material); else the graveyard alone (graveCands).
func (s *State) exileCands(k *prep, p uint8, c *kop.Cost, src, exclude Obj, d *Decision) int {
	if c.ExileHand {
		return s.handExileCands(k, p, c.Filter, exclude, d)
	}
	n := 0
	if c.Flags&kop.CostExileCtrlOrGrave != 0 {
		var fl kop.FilterFlags
		if c.Filter != 0 {
			fl = k.t.Filters[c.Filter].Flags
		}
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			sl := m.first()
			o := s.perms[sl].obj
			if o == exclude || fl&kop.FOther != 0 && o == src || s.picked(o) {
				continue
			}
			if c.Filter != 0 && !s.matchPerm(k, c.Filter, sl, p, NoSlot) {
				continue
			}
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
	return n + s.graveCands(k, p, c.Filter, src, exclude, d)
}

// handExileCands counts (and, with d, offers) p's hand cards matching f not
// already picked, in hand order (gorge exAsk over the hand zone: an
// ExileFromHand<N/Spec> cost; exclude, a card being cast, is never its own
// fodder).
func (s *State) handExileCands(k *prep, p uint8, f kop.FilterID, exclude Obj, d *Decision) int {
	pl := &s.pl[p]
	n := 0
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if o == exclude || s.picked(o) {
			continue
		}
		if f != 0 && !s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), p, o) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}
