package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A restricted source beside a granted mana ability (Creeping Peeper under a
// "creatures have {T}: add {W}" grant): gorge's census lists the printed
// restricted ability (RestrictValid$ since 85fddcb68), so the "choose a mana
// ability" ask of the prefix search offers it first and the script taps it,
// floating its restricted mana, before the grant's alternatives. The source's
// pcard holds no plain own ability (ownManaUsable is false for it), so this
// is the dual branch's printed alternative. Cold, out of line.

// ownRestrAb is the printed restricted mana ability of the table-restricted
// source at sl, nil for any other source.
//
//go:noinline
func (s *State) ownRestrAb(k *prep, sl Slot) *kop.Ability {
	c := s.cardAt(sl)
	if !k.restrSrc || k.gen.flags[c]&gfRestrSrc == 0 || s.genLost(k, sl) {
		return nil
	}
	rec := k.gen.restrRecOf(c)
	if rec == nil {
		return nil
	}
	return &k.t.Cards[c].Abilities[rec.ab]
}
