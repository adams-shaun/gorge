package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static build arms (were arms of (*cardBuilder).static in build_ab.go): the
// block and attack restrictions and permissions.
func init() {
	registerStaticBuild("CantBlock", staticFilterArm(kop.StatCantBlock))
	registerStaticBuild("Lure", staticFilterArm(kop.StatLure)) // combat_lure.go isLure
	registerStaticBuild("CantBlockBy", cbCantBlockByStatic)
	registerStaticBuild("CantAttack", cbCantAttackStatic)
	registerStaticBuild("CantAttackUnless", cbCantAttackUnlessStatic) // lower_attacktax.go
	registerStaticBuild("CantBlockUnless", cbCantBlockUnlessStatic)   // lower_blocktax.go
	registerStaticBuild("MinMaxBlocker", staticFilterArm(kop.StatMaxBlockers))
	registerStaticBuild("CanAttackIfHaste", cbCanAttackIfHasteStatic) // static_canattackifhaste.go
	registerStaticBuild("ExtraBlock", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatExtraBlock // combat_extrablock.go blockCap
		st.A = int16(s.BlockAmount)  // kop.ExtraBlocksAny: combat_blockany.go
		var err error
		st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield)
		return nil, err
	})
	registerStaticBuild("CanAttackDefender", staticFilterArm(kop.StatCanAttackDefender))
	registerStaticBuild("LethalByPower", staticFilterArm(kop.StatLethalByPower)) // static_lethalbypower.go isLethalByPower (Zilortha)
}

func cbCantBlockByStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCantBlockBy
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	st.PwVsSrc = s.PwVsSrc
	return nil, nil
}

func cbCantAttackStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCantAttack
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	return nil, atkScopeBits(st, "CantAttack", s.Flags)
}

func cbCantAttackUnlessStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatAttackTax
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if err := cb.staticAmounts(st, s.Amount, lValue{}); err != nil {
		return nil, err
	}
	return nil, atkScopeBits(st, "CantAttackUnless", s.Flags)
}

func cbCantBlockUnlessStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatBlockTax
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if s.F2 != nil {
		if st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield); err != nil {
			return nil, err
		}
	}
	return nil, cb.staticAmounts(st, s.Amount, lValue{})
}

func cbCanAttackIfHasteStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCanAttackIfHaste
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	for _, fl := range s.Flags {
		if fl != "OppDef" {
			return nil, fmt.Errorf("CanAttackIfHaste flag %q", fl)
		}
		st.StFlags |= kop.StHasteOppDef
	}
	return nil, nil
}
