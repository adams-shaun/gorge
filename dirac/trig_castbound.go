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
		return s.matchCardGenAt(k, f, e.card, s.objs[e.obj].owner(), l.ctrl, e.obj, &cx)
	}
	// the cast card's owner, which is not its caster when a permission cast it
	// from another player's graveyard (Nita, Forum Conciliator's YouDontOwn)
	return s.matchCardTgt(k, f, e.card, s.objs[e.obj].owner(), l.ctrl, e.obj, l.sl, l.obj)
}
