package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Spend-restricted mana from a source whose cost is more than {T}.
//
// A restricted {T}, Pay N life ability of a source with no mana ability of its
// own (Gnarlroot Trapper's Elf-only {G}, Untaidake's legendary-only {C}{C})
// is a last-resort planner unit: gorge plans it as TierLastResort with the
// life a consequence (pay.PaymentPlanShapeTierOf), spent only after every
// other unit, the life paid as the unit is used (CR 119.4, 601.2g), and its
// mana floats restricted (CR 106.6).

// DebugRestrictedLifeFirst is the mutation hook of the restricted last-resort
// tier (dirac/oracle TestRestrictedManaLastResortMutation): a restricted
// {T}, PayLife unit is spent as a normal one, before the other units, instead
// of after them as gorge's TierLastResort does.
var DebugRestrictedLifeFirst bool

// restrLifeAb: a is a restricted "{T}, Pay N life" mana ability.
func restrLifeAb(a *kop.Ability) bool { return kop.RestrictedLifeShape(a) }

// restrSacSelfAb: a is a restricted "{T}, Sacrifice this" mana ability, a
// sacrificing last-resort unit like a Treasure's (pcard.manaSac).
func restrSacSelfAb(a *kop.Ability) bool { return kop.RestrictedSacSelfShape(a) }

// restrLifeUnit: pc is the view of a restricted last-resort life ability.
func restrLifeUnit(pc *pcard) bool { return pc.restrLife != 0 }

// restrLifeIrr is the irreversible cost of a unit whose view is pc: base plus
// 3 a point of a restricted life unit's life (pay.ConsequenceCost), and the
// life its activation pays (0: none).
func restrLifeIrr(pc *pcard, base uint8) (irr, life uint8) {
	if !restrLifeUnit(pc) {
		return base, 0
	}
	if DebugRestrictedLifeFirst {
		return base, pc.restrLife // ranked as a free unit: key 1 no longer holds it last
	}
	return base + 3*pc.restrLife, pc.restrLife
}

// payUnitPain pays the pain of the planned unit of type ty at sl: a
// restricted unit's paid life (CR 119.4), else a painful unit's damage.
func (s *State) payUnitPain(k *prep, sl Slot, p uint8, ty *utype) {
	if ty.life {
		s.loseLife(k, p, int(ty.pain))
		return
	}
	s.painDamage(k, sl, p, int(ty.pain))
}

// restrLifePayable: record rec's life cost (a restricted {T}, Pay N life
// unit) leaves p above 0 life. gorge's planner withholds a last-resort
// alternative whose life reaches the payer's (pay.PlanLastResortChoices,
// CR 119.4); a record with no life cost is payable.
func (s *State) restrLifePayable(k *prep, p uint8, rec *restrRec) bool {
	n := k.gen.rs.views[rec.view].restrLife
	return n == 0 || int(s.pl[SeatIx(p)].life) > int(n)
}

// restrLifeOn: some restricted unit pays life, so the planner's lethal guard
// reads the payer's life even on a board with no painful source.
func restrLifeOn(k *prep) bool { return k.gen != nil && k.gen.rs != nil && k.gen.rs.life }

// restrLifeLast: pc is a restricted life unit plan leaves to phase 1, spent
// only after every other unit (gorge's TierLastResort).
func restrLifeLast(pc *pcard) bool { return restrLifeUnit(pc) && !DebugRestrictedLifeFirst }
