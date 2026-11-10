package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A SpellCast trigger's ValidCard$ bound that reads a value (Filter.PredV:
// Nyota Uhura, Communications' cmcEQChosen, Chalice of the Void's cmcEQY over
// its charge counters) is read with the trigger's source as the source:
// gorge matches the spell through specCtx(host), whose Resolve evaluates the
// bound against the host's own chosen number and SVars. The entry being
// built, the kernel's default context (ctxCur), is the spell cast.

// prepCastBound sets castBoundSrc: some SpellCast trigger's filter reads a
// value.
func prepCastBound(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && tableFilterDynamic(t, a.Trig.Filter) {
				g.castBoundSrc = true
				return
			}
		}
	}
}

// tableFilterDynamic is featFilterDynamic over a table.
func tableFilterDynamic(t *kop.Table, f kop.FilterID) bool {
	for n := 0; f != 0 && int(f) < len(t.Filters) && n < 64; n++ {
		if featPredVDynamic(t.Filters[f].Pred) {
			return true
		}
		f = t.Filters[f].Or
	}
	return false
}

// castFilterOK matches spell e against listener l's SpellCast filter f.
func (s *State) castFilterOK(k *prep, f kop.FilterID, e *stackEntry, l trigListener) bool {
	if k.gen.castBoundSrc && featFilterDynamic(k, f) && !filterChosenType(k.t, f) {
		cx := evalCtx{src: l.obj, srcSl: l.sl, ctrl: l.ctrl}
		return s.matchCardGenAt(k, f, e.card, s.castOwner(k, f, e), l.ctrl, e.obj, &cx)
	}
	return s.matchCardTgt(k, f, e.card, s.castOwner(k, f, e), l.ctrl, e.obj, l.sl, l.obj)
}

// castOwner is the owner slot of the spell e matched against filter f. The
// matchers read one slot for both a filter's controller and owner selector, so
// the caster fills it (the spell's controller on the stack) unless the filter
// names an owner alone and the card's owner differs, as when a permission cast
// another player's card from their graveyard (Nita, Forum Conciliator's
// YouDontOwn).
func (s *State) castOwner(k *prep, f kop.FilterID, e *stackEntry) uint8 {
	if int(e.obj) >= len(s.objs) || s.objs[e.obj].zone != kop.ZoneStack || s.objs[e.obj].owner() == e.ctrl {
		return e.ctrl
	}
	for n := 0; f != 0 && int(f) < len(k.t.Filters) && n < 64; n++ {
		if k.t.Filters[f].Ctrl != kop.CtrlAny {
			return e.ctrl
		}
		f = k.t.Filters[f].Or
	}
	return s.objs[e.obj].owner()
}
