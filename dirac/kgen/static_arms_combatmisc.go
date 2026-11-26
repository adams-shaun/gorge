package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static build arms (were arms of (*cardBuilder).static in build_ab.go):
// attack requirements, combat damage and the permanent-level markers beside
// them.
func init() {
	registerStaticBuild("MustAttack", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, mustAttackStatic(s, st, cb.staticFilter)
	})
	registerStaticBuild("AttackRequirement", cbAttackRequirementStatic)
	registerStaticBuild("Goad", staticFilterArm(kop.StatGoad)) // lower_trigger.go's Continuous Goad$ True
	registerStaticBuild("CombatDamageToughness", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, cb.combatAmountFilter(st, kop.StatCombatDmgToughness, s.F)
	})
	registerStaticBuild("AssignNoCombatDamage", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, cb.combatAmountFilter(st, kop.StatNoCombatDamage, s.F)
	})
	// the as-unblocked election's static (CR 510.1c): the filter reads its
	// creature on the battlefield, and the kernel asks the creature's
	// controller the election in the combat damage step (combat_asgngrant.go)
	registerStaticBuild("AssignCombatDamageAsUnblocked", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		return nil, cb.combatAmountFilter(st, kop.StatAssignCombatDamageAsUnblocked, s.F)
	})
	registerStaticBuild("CantPreventDamage", cbCantPreventDamageStatic)                 // static_cantprevent.go
	registerStaticBuild("ActivateAbilityAsIfHaste", staticFilterArm(kop.StatAsIfHaste)) // static_asifhaste.go
	registerStaticBuild("UntapOtherPlayer", staticFilterArm(kop.StatUntapOther))
	registerStaticBuild("DisableTriggers", staticFilterArm(kop.StatDisableETB))
}

// cbAttackRequirementStatic is the whole-declaration closure (War's Toll):
// Filter is the members ValidCard$ binds, Filter2 the ValidAttacker$
// creatures whose attacking turns the requirement on
// (dirac/combat_attackrequirement.go).
func cbAttackRequirementStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatAttackRequirement
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield)
	return nil, err
}

func cbCantPreventDamageStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatCantPreventDamage
	for _, fl := range s.Flags {
		switch fl {
		case "NoPrevCombat":
			st.StFlags |= kop.StNoPrevCombat
		case "NoPrevSpellSelf":
			st.StFlags |= kop.StNoPrevSpellSelf
		default:
			return nil, fmt.Errorf("CantPreventDamage flag %q", fl)
		}
	}
	return nil, nil
}
