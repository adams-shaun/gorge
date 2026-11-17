package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An announce-bound discard cost: a filtered Discard<1/Card.cmcEQX> beside a
// printed {X} (Knollspine Invocation). CR 601.2b announces X before CR 601.2h
// chooses the discard, so the ask offers only the values some hand card
// matches at; gorge's xAsk drops the rest (announceBoundDiscardPayableAt).

// xDiscardBound reports whether the entry being built discards a filtered
// card whose filter an X bounds.
//
//go:noinline
func (s *State) xDiscardBound(k *prep) bool {
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&kop.CostDiscard == 0 || pc.Flags&kop.CostDiscardFiltered == 0 || pc.DiscardX {
		return false
	}
	for f := pc.Filter; f != 0; f = k.t.Filters[f].Or {
		if k.t.Filters[f].PredV.K == kop.VX {
			return true
		}
	}
	return false
}

// xDiscardPays reports whether the controller's hand pays the discard once the
// announced X is x, the filter read at x through the offer probe.
//
//go:noinline
func (s *State) xDiscardPays(k *prep, e *stackEntry, x int) bool {
	exclude := NoObj
	if e.kind == seSpell {
		exclude = e.obj
	}
	saved := s.g.probe
	s.g.probe.x = uint8(x + 1)
	ok := s.discardPayable(k, e.ctrl, s.buildPickCost(k), exclude)
	s.g.probe = saved
	return ok
}
