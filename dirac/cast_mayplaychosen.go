package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// matchMayPlayFilter is matchCardGenAt for a may-play static's filter. A
// ChosenType alternative (Realmwalker's "creature spells of the chosen type
// from the top of your library") reads the static source permanent's recorded
// choice, a printed changeling having every type (gorge sourceChosen, CR
// 205.3m); matchCardGenAt has no source slot to read it from and would refuse
// every card.
func (s *State) matchMayPlayFilter(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, st statSrc, ctx *evalCtx) bool {
	if st.sl == NoSlot || !filterChosenType(k.t, f) {
		return s.matchCardGenAt(k, f, c, owner, ctrl, o, ctx)
	}
	if ctx != nil && ctx.uncounted == o+1 {
		return false
	}
	if o != NoObj && s.g.tli&tliOn == 0 && s.objs[o].of&ofCopy != 0 && s.copyCeased(k, o) {
		return false
	}
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Flags2&kop.F2ChosenType != 0 {
			ch, ok := s.chosenTypeSub(st.sl)
			cd := &k.t.Cards[c]
			has := ok && int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch))
			if ok && outsideSub(k, ch) {
				has = cd.AllCreatureTypes
			}
			if !has {
				continue
			}
			fl.Flags2 &^= kop.F2ChosenType
		}
		if s.matchCardOneAt(k, &fl, c, owner, ctrl, o, ctx, false) {
			return true
		}
	}
	return false
}
