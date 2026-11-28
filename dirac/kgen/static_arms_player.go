package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Static build arms (were arms of (*cardBuilder).static in build_ab.go): the
// player-level and protection statics, Control and Panharmonicon.
func init() {
	registerStaticBuild("PlayerHexproof", cbPlayerHexproofStatic)
	registerStaticBuild("PlayerShroud", cbPlayerShroudStatic)
	registerStaticBuild("DefenderAssigns", cbDefenderAssignsStatic)
	registerStaticBuild("UntapAdjust", cbUntapAdjustStatic)
	registerStaticBuild("PlayerProtAll", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatPlayerProtAll // the controller has protection from everything (static_plprot.go)
		return nil, nil
	})
	registerStaticBuild("Control", staticFilterArm(kop.StatControl))
	registerStaticBuild("Panharmonicon", cbPanharmoniconStatic)
	registerStaticBuild("ProtYourColors", staticFilterArm(kop.StatProtYourColors)) // kw_protyours.go
	registerStaticBuild("HexproofOwnColors", staticFilterArm(kop.StatHexproofOwnColors))
	// AffectedZone$ All P/T grant (lower_offzonept.go): a marker the kernel's
	// off-battlefield P/T readers stall on (static_offzonept.go)
	registerStaticBuild("OffZonePT", func(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatOffZonePT
		return nil, cb.staticAmounts(st, s.Power, s.Tough)
	})
}

func cbPlayerHexproofStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatPlayerHexproof
	switch {
	case s.Amount.Kind == vNone:
	case s.Amount.isConst() && s.Amount.N == 1:
		st.A = 1 // also protection from each opponent
	default:
		return nil, fmt.Errorf("PlayerHexproof amount %v", s.Amount)
	}
	return nil, nil
}

// cbPlayerShroudStatic: "You have shroud" (Ivory Mask, True Believer,
// Solitary Confinement): A == 2 is the shroud read, the symmetric one of the
// A == 1 protection bit (kernel static_playerstats.go playerShroud; gorge
// playerShroudBlocksTarget). The static is unconditional: a conditioned grant
// is refused at the lowering (the kernel's reader consults no Cond2).
func cbPlayerShroudStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	if s.Cond.Kind != cNone || s.Amount.Kind != vNone {
		return nil, fmt.Errorf("PlayerShroud beside %s", s.Cond)
	}
	st.Kind = kop.StatPlayerHexproof
	st.A = 2
	return nil, nil
}

// cbDefenderAssignsStatic is Defensive Formation (lower_playerstatic.go):
// unconditional, as the kernel read (static_plkeyword.go) consults no Cond2.
func cbDefenderAssignsStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	if s.Cond.Kind != cNone || s.Amount.Kind != vNone {
		return nil, fmt.Errorf("DefenderAssigns beside %s", s.Cond)
	}
	st.Kind = kop.StatDefenderAssigns
	return nil, nil
}

// cbUntapAdjustStatic is Static Orb (lower_playerstatic.go): the cap's N is
// Static.A, the capped permanents are the static's filter (Permanent for the
// one lowered printing), and Static.Cond2 carries IsPresent$ Card.Self+untapped.
// The kernel read is dirac/static_untapadjust.go.
func cbUntapAdjustStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	if !s.Amount.isConst() {
		return nil, fmt.Errorf("UntapAdjust amount %v", s.Amount)
	}
	st.Kind = kop.StatUntapAdjust
	st.A = int16(s.Amount.N)
	var err error
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	return nil, nil
}

func cbPanharmoniconStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatPanharmonicon
	st.PanhZones = s.PanhZones
	st.PanhTurned = s.PanhTurned
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if err = panharmoniconFlags(st, s.Flags); err != nil {
		return nil, err
	}
	if s.F2 != nil {
		// ValidCause$ (the entering permanent): kop.StPanhETB; the dies
		// form (A == PanhModeDies) reads Filter2 as the dying permanent and
		// the turn-up form (PanhTurned) as the turned permanent.
		if st.A != kop.PanhModeDies && !st.PanhTurned {
			st.StFlags |= kop.StPanhETB
		}
		if st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
