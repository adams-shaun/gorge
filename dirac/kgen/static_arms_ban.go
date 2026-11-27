package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static build arms (were arms of (*cardBuilder).static in build_ab.go): the
// "can't" statics that are not combat or cost.
func init() {
	registerStaticBuild("CantBeActivated", cbCantBeActivatedStatic) // lower_cantact.go
	// Ground Seal's class (lower_canttarget.go): every card in every graveyard
	// untargetable (kw_canttgtgrave.go). No operands.
	registerStaticBuild("CantTgtGrave", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatCantTgtGrave
		return nil, nil
	})
	registerStaticBuild("CantTarget", cbTargetStatic) // lower_canttarget.go
	registerStaticBuild("MustTarget", cbTargetStatic) // static_musttarget.go
	registerStaticBuild("CantPhaseIn", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatCantPhaseIn // kw_phasing.go phaseBlocked
		return nil, nil
	})
	registerStaticBuild("CantSacrifice", staticFilterArm(kop.StatCantSacrifice)) // static_cantsacrifice.go
	registerStaticBuild("CantPutCounter", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatCantPutCounter
		var err error
		if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
			return nil, err
		}
		st.A = s.Ctr
		return nil, nil
	})
	registerStaticBuild("CantGainLife", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatCantGainLife
		if s.Amount.Kind != vNone {
			st.A = 1 // the replacement form (lower_lifeprevent.go; static_lifeprevent.go)
		}
		return nil, banOppFlag(s.Flags, st)
	})
	registerStaticBuild("CantDraw", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatCantDraw
		if err := banOppFlag(s.Flags, st); err != nil {
			return nil, err
		}
		if s.Amount.Kind != vNone {
			// DrawLimit$ N as N+1 (static_restrict.go; kernel playerBanned)
			if !s.Amount.isConst() || s.Amount.N < 1 || s.Amount.N > 255 {
				return nil, fmt.Errorf("CantDraw limit %v", s.Amount)
			}
			st.A = int16(s.Amount.N)
		}
		return nil, nil
	})
	registerStaticBuild("CantBeSuspected", staticFilterArm(kop.StatCantBeSuspected)) // act_suspect.go suspectOne
}

func cbCantBeActivatedStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatCantActivate
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	for _, fl := range s.Flags {
		switch fl {
		case "ActivatorYou":
			st.StFlags |= kop.StRaiseYou
		case "ActivatorOpp":
			st.StFlags |= kop.StRaiseOpp
		case "SpareMana":
			st.A |= 1
		case "LoyaltyOnly":
			st.A |= 2
		case "NamedCard":
			st.A |= kop.CantActNamed
		default:
			return nil, fmt.Errorf("cant be activated flag %q", fl)
		}
	}
	return nil, nil
}

// cbTargetStatic is CantTarget and MustTarget, which differ in their kind
// (targetStaticKind) only.
func cbTargetStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = targetStaticKind(s.Kind)
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	for _, fl := range s.Flags {
		switch fl {
		case "ActivatorYou":
			st.StFlags |= kop.StRaiseYou
		case "ActivatorOpp":
			st.StFlags |= kop.StRaiseOpp
		case "SASpell":
			st.A |= kop.CantTgtSpell
		case "SAActivated":
			st.A |= kop.CantTgtActivated
		case "SATriggered":
			st.A |= kop.CantTgtTriggered
		default:
			return nil, fmt.Errorf("cant target flag %q", fl)
		}
	}
	return nil, nil
}
