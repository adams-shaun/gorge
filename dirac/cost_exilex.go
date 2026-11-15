package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An announced ExileFromGrave<X/Spec> on a spell with a printed {X} ("as an
// additional cost, exile X cards from your graveyard", Skeletal Scrying;
// CR 601.2b, 107.3a). The table encodes it as CostExileGrave with Cost.N == 0
// (a literal ExileFromGrave<N/..> has N >= 1, and the CARDNAME singleton
// keeps N 1, so every fixed-exile reader stays correct on it; the same shape
// as CostPayLife N 0, cost_paylifex.go). gorge: the part is cost.CostPart
// Announced; xAsk caps the announcement by the cards that could pay it
// (announcedExileBounds, min-clamping the mana bound), and exAsk asks one
// KChoose of exactly the announced count over the graveyard's matching cards
// in zone order. The kernel's X ask min-clamps by the same candidates and the
// cost-pick stage walks X pkExile picks (act_forage.go). Cold code: the build
// reaches it only behind genPrep.anyExileX.

// prepExileX sets anyExileX.
func prepExileX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if exileXCost(&t.Cards[i].SpellCost) {
			g.anyExileX = true
		}
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbSpell && exileXCost(&a.Cost) {
				g.anyExileX = true // a spell ability carries the spell's own additional cost (spellAddCost)
			}
		}
	}
}

// exileXCost reports whether c exiles the announced X graveyard cards.
func exileXCost(c *kop.Cost) bool {
	return c != nil && c.Flags&kop.CostExileGrave != 0 && c.N == 0 && !c.ExileGSelf && !c.ExileHand && c.Flags&kop.CostExileCtrlOrGrave == 0
}

// exileXClamp bounds the X ask of the spell being built by the graveyard cards
// its exile part could take (a cast card is never its own fodder): the cap
// min-clamps the mana bound mx, which stays negative when nothing is payable.
//
//go:noinline
func (s *State) exileXClamp(k *prep, mx int) int {
	pc := s.buildPickCost(k)
	if !exileXCost(pc) {
		return mx
	}
	e := &s.bld().e
	n := s.graveCands(k, e.ctrl, pc.Filter, e.obj, e.obj, nil)
	return min(mx, min(n, 255))
}
