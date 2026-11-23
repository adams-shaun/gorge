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

// ownRestrAbListed is ownRestrAb for a listing: a gate-off restricted ability
// (its IsPresent$ mana gate fails) is absent from every list, as gorge lists
// it nowhere (mana_gate.go's ManaActivationGateHolds holds for the priority
// offer, the payment window and the planner census; restrDynBesideGrant's
// same check).
//
//go:noinline
func (s *State) ownRestrAbListed(k *prep, sl Slot) *kop.Ability {
	ra := s.ownRestrAb(k, sl)
	if ra != nil && ra.Trig.Cond != 0 && !s.manaGateHolds(k, ra, sl) {
		return nil
	}
	return ra
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

// restrDynBesideGrant is grantRestrOwnFirst for a dynamic amount whose
// restriction the payment may not spend (Woodland Weavemaster's Elf-only mana
// beside Citanul Hierophants' {T}: Add {G}, paying Paralyze's {4}): gorge's
// "choose a mana ability" ask still lists the printed ability ahead of the
// grant, and the window's answer takes it, so the evaluated amount floats
// restricted and the grant is never tapped (CR 106.6). The ability and its
// amount, nil with none.
//
//go:noinline
func (s *State) restrDynBesideGrant(k *prep, sl Slot) (*kop.Ability, uint8) {
	ra := s.ownRestrAb(k, sl)
	if ra == nil || ra.AFlags&kop.AfManaDynamic == 0 || s.ownManaUsable(k, sl) || !s.restrUsable(k, sl) {
		return nil, 0
	}
	if len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || s.windowGrantMask(k, sl) == 0 {
		return nil, 0
	}
	if ra.Trig.Cond != 0 && !s.condEval(k, ra.Trig.Cond, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl}) {
		return nil, 0 // a gated ability is absent (restrViewW)
	}
	n := s.dynManaN(k, ra, sl)
	if n <= 0 {
		return nil, 0
	}
	return ra, uint8(min(n, maxDynMana))
}
