package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The announced PayEnergy<X> of a printed activated ability with no printed
// {X} (Chthonian Nightmare: "Pay X {E}, Sacrifice a creature, Return
// Chthonian Nightmare to its owner's hand: Return target creature card with
// mana value X ..."; CR 107.3, 107.14, 601.2b, 602.2b). gorge: the part
// announces the cast's X (xAsk's energyX), the payer's energy total is the
// bound (Forge CostPayEnergy.getMaxAmountX), the offer gate judges an
// X-bound target pending (costAnnouncesX counts the part: any X may fit) and
// payCast pays the announced X in energy. The table marks the part
// kop.Cost.EnergyX (Cost.N stays the other counted part's). The X ask is the
// ordinary bsX one, its values filtered by the target ask (addXOptions).
// Cold code: the build reaches it only behind the useEnergyX bit.

// DebugEnergyXUnpaid is the mutation switch: the announced energy is not
// paid.
var DebugEnergyXUnpaid bool

// useEnergyX: some activated ability costs an announced PayEnergy<X>.
var useEnergyX = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Cost.EnergyX {
				return true
			}
		}
	}
	return false
})

// energyXBuilding reports whether the entry being built pays the announced X
// in energy.
//
//go:noinline
func (s *State) energyXBuilding(k *prep) bool {
	pc := s.buildPickCost(k)
	return pc != nil && pc.EnergyX
}

// energyXClamp bounds the X ask by the payer's energy: announced only
// (cost.X == 0) the energy is the bound, offered only when X = 0 is payable
// (mx >= 0), as lifeXClamp.
//
//go:noinline
func (s *State) energyXClamp(k *prep, cost *kop.ManaCost, mx int) int {
	if !s.energyXBuilding(k) {
		return mx
	}
	en := int(s.g.energy[SeatIx(s.bld().e.ctrl)])
	if cost.X == 0 {
		if mx < 0 {
			return 0
		}
		return min(en, 255)
	}
	return min(mx, en)
}

// payEnergyX pays the announced x energy of an activation's cost.
//
//go:noinline
func (s *State) payEnergyX(p uint8, x uint8) {
	if DebugEnergyXUnpaid {
		return
	}
	s.payEnergyN(p, x)
}
