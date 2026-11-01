package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// exileCands counts (and, with d, offers) the candidates of an exile cost
// part c: with kop.CostExileCtrlOrGrave, the payer's other permanents
// matching c.Filter (battlefield order) and then their graveyard's cards
// (gorge exAsk over exilePartZones' battlefield, graveyard order; Craft's
// material); else the graveyard alone (graveCands).
func (s *State) exileCands(k *prep, p uint8, c *kop.Cost, src, exclude Obj, d *Decision) int {
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
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	}
	return n + s.graveCands(k, p, c.Filter, src, exclude, d)
}
