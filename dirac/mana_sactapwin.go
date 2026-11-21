package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// windowSacTapOwed: the mana window's answer at sl is sl's LATER
// "{T}, Sacrifice a <spec>: Add <colour>" mana ability (kop.SacTapFixedManaAb
// beside a plain fixed {T} first one, kop.SecondSacTapManaAb — Phyrexian
// Tower), not the plain tap. gorge's ability ask lists the source's mana
// abilities owed first (engine/rules/mana_activation.go owedAbilitiesFirst, a
// stable partition of the printed order: paymentUsefulColours' demand — a
// fixed pip of the pending payment the floating pool still owes — sorts an
// ability ahead) and the answer takes the first listed, so when the sac
// ability's colour is a pip the pool still owes and the plain first ability's
// colours are not ({C} has no coloured pips: its demand reads the cost's {C}
// slot, and a generic-only remainder owes nothing), the answer is the sac
// ability. When both or neither are owed the printed order stands and the
// answer is the plain tap, as before. It returns the sac ability's colour.
//
// Only a window gorge poses picks from takes it: a cast's or an activation's
// mana step (castTapX, the gate windowPick's tap-another arm reads), where the
// walk pauses on the sacrifice pick (castWinTapOptions, ChCostSacrifice) as it
// does for a sacrifice-another source (mana_windowtapx.go windowTapXOffer).
// The trigger-cost, unless and ward windows' asks carry no ManaPayment and
// read no pending cast, so gorge's owedAbilitiesFirst does not reorder there
// and the plain tap stands.
//
//go:noinline
func (s *State) windowSacTapOwed(k *prep, p uint8, sl Slot, cost *kop.ManaCost) (col int, ok bool) {
	if s.g.winMode != winNone || !s.castTapX(k) {
		return 0, false // a resolution window or no live build: the plain tap, as before
	}
	ab := k.tapXAbOf(s.cardAt(sl))
	if ab == nil || !k.tapXSecond(s.cardAt(sl)) || !kop.SacTapFixedManaAb(ab) {
		return 0, false // not the Phyrexian Tower shape
	}
	if cost.Hyb[0] != 0 {
		return 0, false // the pips are announced before the window (CR 601.2b): the cost is resolved
	}
	if !s.scriptTapX(k, p).has(sl) {
		return 0, false // the sac ability cannot be activated now: the window lists the plain tap alone
	}
	var owed kop.ManaCost
	s.windowOwedCost(k, p, cost, &owed)
	sacCol := bits.TrailingZeros8(uint8(ab.ManaColors) & 0x1f)
	if owed.Pips[sacCol] <= s.pl[p].pool[sacCol] {
		return 0, false // the sac colour is not a fixed pip still owed: the printed order stands
	}
	m := uint8(s.manaPCGen(k, sl).manaCol)
	for c := 0; c < kop.NumManaKinds; c++ {
		if m&(1<<uint(c)) != 0 && owed.Pips[c] > s.pl[p].pool[c] {
			return 0, false // the plain ability's colour is owed too: the stable partition keeps the printed order
		}
	}
	return sacCol, true
}
