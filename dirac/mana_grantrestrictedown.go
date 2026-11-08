package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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

// grantRestrOwnFirst: the cast window's "choose a mana ability" ask of sl, a
// table-restricted source whose only own ability is its printed restricted
// one beside a grant (Cargo Ship's artifact-only {C} under A Realm Reborn),
// lists that printed ability first, and the answer takes it, whether or not
// the payment may spend its mana (CR 106.6: the mana floats restricted).
// gorge's owedAbilitiesFirst moves the grant ahead only when the grant makes
// a colour a fixed pip still owes (want) and the printed ability does not.
// Only a single-colour printed ability: an any-colour one's ordering reads
// its admission, which windowRestrOwed's own walk answers. A dynamic amount
// is left to windowRestrDyn (lockstepconfig.go), which floats its evaluated
// amount: kgen's ManaN is the placeholder 1 for every dynamic ability.
func (s *State) grantRestrOwnFirst(k *prep, sl Slot, want uint8) (*kop.Ability, bool) {
	ra := s.ownRestrAb(k, sl)
	if ra == nil || ra.AFlags&kop.AfManaDynamic != 0 || s.ownManaUsable(k, sl) || bits.OnesCount8(uint8(ra.ManaColors)&0x3f) != 1 {
		return nil, false
	}
	if len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) {
		return nil, false
	}
	gm := s.windowGrantMask(k, sl)
	if gm == 0 {
		return nil, false
	}
	if uint8(ra.ManaColors)&want == 0 && gm&want != 0 {
		return nil, false // the grant owes and the printed ability does not: the grant first
	}
	return ra, true
}
