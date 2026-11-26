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
// cost-pick stage walks X pkExile picks (act_forage.go). A printed activated
// ability carries the part too, beside a printed {X} (Necropolis Fiend) or
// alone (Taigam, Sidisi's Hand): announced only, gorge's xAsk sets the bound
// to the candidates (applyCap's first announced part) and poses the ask even
// with no {X}; its offer gate (costAnnouncesX counts no announced Exile part)
// judges an X-bound target at X = 0, as for an announced Sac<X/Spec>. Cold
// code: the build reaches it only behind genPrep.anyExileX.

// prepExileX sets anyExileX.
func prepExileX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if exileXCost(&t.Cards[i].SpellCost) {
			g.anyExileX = true
		}
		for j := range t.Cards[i].Abilities {
			if exileXCost(&t.Cards[i].Abilities[j].Cost) {
				g.anyExileX = true // a spell ability carries the spell's own additional cost (spellAddCost); an activated ability its own
			}
		}
	}
}

// exileXCost reports whether c exiles the announced X graveyard cards.
func exileXCost(c *kop.Cost) bool {
	return c != nil && c.Flags&kop.CostExileGrave != 0 && c.N == 0 && !c.ExileGSelf && !c.ExileHand && c.Flags&kop.CostExileCtrlOrGrave == 0
}

// exileXClamp bounds the X ask of the entry being built by the graveyard cards
// its exile part could take (a cast card is never its own fodder): beside a
// printed {X} the cap min-clamps the mana bound mx, which stays negative when
// nothing is payable; announced only (cost.X == 0) the count is the bound,
// offered only when X = 0 is payable (mx >= 0), as sacXClamp.
//
//go:noinline
func (s *State) exileXClamp(k *prep, cost *kop.ManaCost, mx int) int {
	pc := s.buildPickCost(k)
	if !exileXCost(pc) {
		return mx
	}
	e := &s.bld().e
	n := min(s.graveCands(k, e.ctrl, pc.Filter, e.obj, e.obj, nil), 255)
	if cost.X == 0 {
		if mx < 0 || DebugExileXAnnouncedOnly {
			return 0
		}
		return n
	}
	return min(mx, n)
}

// DebugExileXAnnouncedOnly is the mutation switch: an announced-only
// ExileFromGrave<X/Spec>'s X ask offers X = 0 alone.
var DebugExileXAnnouncedOnly bool

// exileXBuilding reports whether the entry being built exiles the announced
// X graveyard cards.
//
//go:noinline
func (s *State) exileXBuilding(k *prep) bool {
	return exileXCost(s.buildPickCost(k))
}
