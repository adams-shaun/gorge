package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The plane-source curse record: Eloren Wilds' chaos body's DB$ Effect
// ("Whenever chaos ensues, target player can't cast spells until a player
// planeswalks"). gorge lowers the Effect to a registered CantBeCast
// restriction whose Duration is the UntilPlaneswalk lifetime (engine/effects
// effEffect's effectPlaneCurseEnd; rules' checkPlaneswalkTriggers ends
// every live registration at the next PlanarWalk through
// withoutUntilPlaneswalk). The kernel carries no effect token either, so
// the narrow equivalent lives on the cold planar record (zone_planechase.go's
// planeSeat, present in every table's layout): one bit per target seat. The
// predicate is target-only -- the static's Caster$ Player.IsRemembered names
// the captured player, ValidCard$ Card admits every card, and the source and
// controller a registration carries are read by no clause of this shape.

// planeCurseSet marks seat p cursed (ExtPlaneCurse).
func (s *State) planeCurseSet(p uint8) {
	if c := s.planeCold(); c != nil && int(p) < MaxSeats {
		c.curse[p] = true
	}
}

// planeCurseClear ends every live curse: any player's planeswalk.
func (s *State) planeCurseClear() {
	if c := s.planeCold(); c != nil {
		for p := range c.curse {
			c.curse[p] = false
		}
	}
}

// planeCurseBlocks reports whether the cast gate forbids seat p: a live curse
// over p. gorge's castRestrictionSources consults the registered restriction
// after the face statics, so castForbiddenCost reads the record after the
// static scan (cast_cantself.go). The CR 601.2e recheck skips it in the
// kernel because it skips statics whose filter reads no mana value, and
// ValidCard$ Card reads none -- gorge's recheck never re-answers it either.
func (s *State) planeCurseBlocks(p uint8) bool {
	c := s.planeCold()
	return c != nil && int(p) < MaxSeats && c.curse[p]
}

// tableHasPlaneCurse reports whether some card of t lowers a PlaneCurse
// instr. The curse's cast gate rides the same arm a StatCantBeCast table
// arms (gen.go's anyCantCast: the offer walks skip castForbiddenCost for a
// table the flag does not name), so the registration opens it.
func tableHasPlaneCurse(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtPlaneCurse {
					return true
				}
			}
		}
	}
	return false
}

func init() { genNeed(tableHasPlaneCurse) }

// planeCurseExt is ExtPlaneCurse: the DB$ Effect's target player is cursed
// until a player planeswalks.
//
//kmanifest:ext ExtPlaneCurse
func (s *State) planeCurseExt(k *prep, in *kop.Instr) (bool, bool) {
	if p := s.playerRef(k, in.Obj); p != 255 {
		s.planeCurseSet(p)
	}
	return true, true
}
