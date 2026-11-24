package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for damage, fights and damage prevention (instr.go).
func init() {
	instrOp("DealDamage", (*instrCtx).opDealDamage)
	instrOp("SweepFrom", (*instrCtx).opSweepFrom)
	instrOp("DamageAll", (*instrCtx).opDamageAll)
	instrOp("ResolveDamageMap", (*instrCtx).opResolveDamageMap)
	instrOp("DealDamageDivided", (*instrCtx).opDealDamageDivided)
	instrOp("DealDamageFrom", (*instrCtx).opDealDamageFrom)
	instrOp("Bite", (*instrCtx).opBite)
	instrOp("Fight", (*instrCtx).opBite)
	instrOp("EachDamage", (*instrCtx).opEachDamage)
	instrOp("DmgMark", (*instrCtx).opDmgMark)
	instrOp("DmgRedirect", (*instrCtx).opDmgRedirect)
	instrOp("PreventColor", (*instrCtx).opPreventColor)
	instrOp("PreventDamageScope", (*instrCtx).opPreventDamageScope)
	instrOp("PreventNext", (*instrCtx).opPreventNext)
	instrOp("ChooseSource", (*instrCtx).opChooseSource) // lower_choosesource.go
	instrOp("Fog", (*instrCtx).opFog)
}

func (ic *instrCtx) opDealDamage() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	ki.Op = kop.OpDealDamage
	if in.Obj.Kind == rDefending && in.Obj.N == 1 {
		ki.B = kop.Const(kop.DmgAtkedObj) // lower_tokenattacking.go: the attacked planeswalker/battle too
	}
	if in.F != nil {
		if err := ic.objFilter(zBattlefield); err != nil {
			return err
		}
	}
	c := 0
	if _, ok := fl["TrackExcess"]; ok {
		c |= kop.DmgTrackExcess
	}
	if _, ok := fl["ExileIfDiesThisTurn"]; ok {
		c |= kop.DmgExileIfDiesThisTurn
	}
	if _, ok := fl["Remember"]; ok {
		c |= kop.DmgRemember
	}
	if _, ok := fl["ExileIfDealt"]; ok {
		c |= kop.DmgExileIfDealt
	}
	if _, ok := fl["NoRegen"]; ok {
		c |= kop.DmgNoRegen
	}
	if _, ok := fl["NoRegenTarget"]; ok {
		c |= kop.DmgNoRegenTarget
	}
	if _, ok := fl["NoPrevention"]; ok {
		c |= kop.DmgNoPrevention // lower_noprevention.go
	}
	if strings.HasPrefix(in.Note, "damage map") || cb.inDamageMap {
		c |= kop.DmgMapped
		cb.inDamageMap = true
	}
	if c != 0 && ki.B.N == kop.DmgAtkedObj {
		return refuse("damage to the defender or attacked planeswalker with a DealDamageEx flag")
	}
	if c != 0 {
		ki.Op, ki.C = kop.OpDealDamageEx, int16(c)
	}
	return nil
}

func (ic *instrCtx) opSweepFrom() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtSweepFrom over Obj, B the amount, C&1 the
	// source's own slot excluded, C&2 each player after the creatures
	// (a capturing emblem's DamageSource$ Remembered sweep)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSweepFrom)), int16(in.C)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opDamageAll() error {
	ki := &ic.ki
	in := &ic.in
	if _, ok := ic.fl["OtherOpps"]; ok {
		// ValidPlayers$ OppNonTriggeredTarget (lower_effect.go damageAll):
		// B.N bit 32, no permanent, each opponent but the trigger's player
		// (kernel damageOtherOpps)
		ki.Op, ki.B = kop.OpDamageAll, kop.ValueRef{N: 32}
		return nil
	}
	switch {
	case in.F != nil && len(in.F.Clauses) == 0 && in.F.Players == plBoth:
		// "N damage to each player".
		ki.Op, ki.Obj = kop.OpDealDamage, kop.RefEachPlayer
	case in.F != nil && len(in.F.Clauses) == 0 && in.F.Players == plOpp:
		// "N damage to each opponent" (Grab the Prize's ValidPlayers$
		// Player.Opponent; seat-exact: the kernel deals each opponent)
		ki.Op, ki.Obj = kop.OpDealDamage, kop.RefEachOpponent
	case in.F != nil && in.F.Players == 0 && in.Obj.Kind == rTarget:
		// ValidPlayers$ Targeted (lower_damagealltargeted.go): B.N bit 8 the
		// targeted player in Obj, bit 16 the sweep scoped to that player's
		// permanents; bit 4 as below
		ki.Op = kop.OpDamageAll
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
		b := int16(8)
		if _, ok := ic.fl["Scope"]; ok {
			b |= 16
		}
		for _, c := range in.F.Clauses {
			if c.Types&(tPlaneswalker|tBattle) != 0 {
				b |= 4
			}
		}
		ki.B = kop.ValueRef{N: b}
	case in.F != nil && in.F.Players == 0:
		ki.Op = kop.OpDamageAll
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
		// a planeswalker or battle clause ("each creature and each
		// planeswalker that player controls", Incinerator of the
		// Guilty): B.N bit 4, the filter's types choose (gorge
		// effDamageAll damages every matching permanent; CR 120.3c)
		for _, c := range in.F.Clauses {
			if c.Types&(tPlaneswalker|tBattle) != 0 {
				ki.B = kop.ValueRef{N: 4}
			}
		}
	case in.F != nil && in.F.Players == plOpp && len(in.F.Clauses) > 0:
		// "each opponent and each creature and planeswalker they
		// control" (End the Festivities): the permanents, then the
		// opponent (B.N bit 2); bit 4 when a clause names a
		// non-creature type, so the filter's types choose
		f := *in.F
		f.Players = 0
		in.F = &f
		ki.Op = kop.OpDamageAll
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
		b := int16(2)
		for _, c := range f.Clauses {
			if c.Types&^tCreature != 0 || c.Types == 0 {
				b |= 4
			}
		}
		ki.B = kop.ValueRef{N: b}
	case in.F != nil && in.F.Players == plBoth && len(in.F.Clauses) > 0:
		// "each creature and each player" (Encounter the Unknown): the
		// creatures, then each player, the controller first (B.N bit 1)
		for _, c := range in.F.Clauses {
			if c.Types != tCreature {
				return fmt.Errorf("DamageAll filter %s: each player beside a non-creature clause", in.F)
			}
		}
		f := *in.F
		f.Players = 0
		in.F = &f
		ki.Op = kop.OpDamageAll
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
		ki.B = kop.ValueRef{N: 1}
	default:
		return fmt.Errorf("DamageAll filter %s", in.F)
	}
	if ki.Op == kop.OpDealDamage {
		ic.skipFilterCheck = true
		return nil
	}
	return nil
}

func (ic *instrCtx) opResolveDamageMap() error {
	ki := &ic.ki
	cb := ic.cb
	ki.Op = kop.OpResolveDamageMap
	cb.inDamageMap = false
	return nil
}

func (ic *instrCtx) opDealDamageDivided() error {
	ki := &ic.ki
	fl := ic.fl
	ki.Op = kop.OpDealDamageDivided
	if _, ok := fl["DivideEvenly"]; ok {
		ki.C = kop.DmgDivideEvenly
	}
	return nil
}

func (ic *instrCtx) opDealDamageFrom() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpDealDamageFrom
	if in.CS == "players" {
		// each-opponent recipients (lower_damagesourceplayers.go): B.N 1 is the
		// kernel's player mode (dirac/act_damagefrom.go)
		ki.C, ki.B = int16(kop.RefEachOpponent), kop.Const(1)
		return nil
	}
	if in.CS == "defending" {
		// the Attacks trigger's defending player (lower_damagesourceplayers.go):
		// the player mode over RefDefendingPlayer (dirac eachPlayers -> oneOpp)
		ki.C, ki.B = int16(kop.RefDefendingPlayer), kop.Const(1)
		return nil
	}
	if in.CS == "eachctrl" {
		// the RepeatEach card deals to its own controller
		// (lower_repeatcardsdamage.go): the player mode's B.N DmgFromEachCtrl
		ki.C, ki.B = 0, kop.Const(kop.DmgFromEachCtrl)
		return nil
	}
	if in.CS == "anytgt" {
		// the slot may name a player or a permanent (lower_damagesourceplayers.go)
		r, err := cb.ref(lRef{Kind: rTarget, N: in.C})
		if err != nil {
			return err
		}
		ki.C, ki.B = int16(r), kop.Const(1)
		return nil
	}
	r, err := cb.ref(lRef{Kind: rTarget, N: in.C})
	if err != nil {
		return err
	}
	ki.C = int16(r)
	return nil
}

func (ic *instrCtx) opBite() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	ki.Op = kop.OpBite
	if in.Op == "Fight" {
		ki.Op = kop.OpFight
		if _, ok := fl["TrackExcess"]; ok {
			ki.A = kop.Const(1) // ExcessSVar$: record ExExcessDamage for the hit dealt to C (dirac/value_fightexcess.go)
		}
	}
	r, err := cb.ref(lRef{Kind: rTarget, N: in.C})
	if err != nil {
		return err
	}
	ki.C = int16(r)
	return nil
}

func (ic *instrCtx) opEachDamage() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// (Case of the Gateway Express) OpExt ExtEachDamage
	// C: the recipient Ref (low byte) | the damager target slots << 8
	// (0 = the F sweep)
	r, err := cb.ref(in.Obj2)
	if err != nil {
		return err
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtEachDamage)), int16(r)|int16(in.C)<<8
	return nil
}

func (ic *instrCtx) opDmgMark() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtDmgMark, Obj the damaged player, or C kop.DmgMarkBySource
	// for the resolving controller's sources (dirac/repl_dmgmark.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtDmgMark)), int16(in.C)
	return nil
}

func (ic *instrCtx) opDmgRedirect() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtDmgRedirect, Obj the recipient, C the kop.DmgRedirect* kind (dirac/repl_dmgredirect.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtDmgRedirect)), int16(in.C)
	return nil
}

func (ic *instrCtx) opPreventColor() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPreventColor)) // (Prismatic Strands)
	return nil
}

func (ic *instrCtx) opPreventDamageScope() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtPreventDamageScope)), int16(in.C)
	if in.C&kop.PrevScopeFiltered == 0 {
		return nil
	}
	// F the damage source's filter, B the recipient permanents' (kop.PrevScopeFiltered)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	if in.F2 != nil {
		id, err := ic.cb.filter(in.F2, kop.ZoneBattlefield)
		if err != nil {
			return err
		}
		if id > 1<<15-1 {
			return fmt.Errorf("PreventDamageScope recipient filter %d beyond a ValueRef", id)
		}
		ki.B = kop.Const(int(id))
	}
	return nil
}

func (ic *instrCtx) opPreventNext() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	// OpExt ExtPreventNext on Obj, B the amount, C the kop.PrevNext* flag
	// bits: 1 combat damage only, 2 all damage until the recipient leaves, 4
	// keyed on the damage source, 8 the gain rider, 16 the token rider. The
	// upper bits name the multi-recipient and rider kinds (kop/ext_prevnext.go);
	// W is the rider's target or the divided target slot. With 16 (the token
	// rider) B is the named token's pool index instead, mapped from the
	// lInstr's CS key: the amount is unread for the unbounded shield, and the
	// kernel's rider mints CardID(B) of it.
	if in.C&kop.PrevNextTokens != 0 {
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return fmt.Errorf("token %q", in.CS)
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtPreventNext)), kop.Const(idx), int16(in.C)
		return nil
	}
	if in.F != nil {
		// the multi-recipient shape loops the battlefield with this filter
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	v, err := cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtPreventNext)), v, int16(in.C)
	return nil
}

func (ic *instrCtx) opFog() error {
	ki := &ic.ki
	ki.Op = kop.OpFog
	return nil
}
