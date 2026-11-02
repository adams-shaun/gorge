package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugCastLimitOff is TestCastLimitAndGrantedLookMutation's switch: the NumLimitEachTurn
// cap is ignored (the static forbids every cast).
var DebugCastLimitOff bool

// prepCastLimit: a CantBeCast static with a NumLimitEachTurn cap (A>0)
// reads the history bank's spell count, so it turns anyHist on.
func prepCastLimit(t *kop.Table, g *genPrep) {
	capped := func(abs []kop.Ability) bool {
		for j := range abs {
			if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantBeCast && a.Static.A > 0 {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		if capped(t.Cards[i].Abilities) {
			g.anyHist = true
		}
	}
	for i := range t.Emblems {
		if capped(t.Emblems[i].Abilities) {
			g.anyHist = true
		}
	}
}

// castLimitOpen reports whether a capped CantBeCast static (A>0) does not
// bind yet: p has cast fewer than A spells this turn (gorge
// castLimitBinds; histSpells excludes the candidate, offer-time).
func (s *State) castLimitOpen(st *kop.Static, p uint8) bool {
	return st.A > 0 && !DebugCastLimitOff && int(s.g.hist[p][histSpells]) < int(st.A)
}
