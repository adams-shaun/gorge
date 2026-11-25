package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for life, poison, blight and the game's end (lower_api.go).
func init() {
	apiOp("GainLife", (*lowerCtx).apiGainLife)
	apiOp("LoseLife", (*lowerCtx).apiGainLife)
	apiOp("LosesGame", (*lowerCtx).apiLosesGame)
	apiOp("WinsGame", (*lowerCtx).apiLosesGame)
	apiOp("SetLife", (*lowerCtx).apiSetLife)
	apiOp("Extort", (*lowerCtx).apiExtort)
	apiOp("Poison", (*lowerCtx).apiPoison)
	apiOp("Radiation", (*lowerCtx).apiRadiation)
	apiOp("Blight", (*lowerCtx).apiBlight)
}

func (lc *lowerCtx) apiGainLife(sa *cards.SA, ps *params) error {
	// Expel the Unworthy's split target: the root's "instead" bound resolves
	// to zero targets when kicked (TargetMin$/Max$ X=0), while the Kicked-only
	// ChangeZone sub chooses slot 1; unkicked the root chooses slot 0 and the
	// sub is skipped. The trailing gain names "its controller" and "its mana
	// value" of whichever creature was actually exiled, so lower one guarded
	// arm per kicker outcome reading that slot. Without this the single
	// lastTarget() read always named slot 1, which exists only when kicked.
	if sa.API == "GainLife" && !dirac.DebugExpelGainTarget1 {
		if unkicked, kicked, ok := lc.kickedSplitGain(ps); ok {
			lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cKicked, Neg: true}, Skip: 1})
			lc.emit(lInstr{Op: sa.API, Obj: lRef{Kind: rTargetController, N: unkicked},
				A: lValue{Kind: vMVOf, Ref: lRef{Kind: rTarget, N: unkicked}}})
			lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cKicked}, Skip: 1})
			lc.emit(lInstr{Op: sa.API, Obj: lRef{Kind: rTargetController, N: kicked},
				A: lValue{Kind: vMVOf, Ref: lRef{Kind: rTarget, N: kicked}}})
			return nil
		}
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	var n lValue
	if sa.API == "GainLife" && strings.TrimSpace(ps.str("LifeAmount")) == "AFLifeLost" && lc.followsLoseLife() {
		// Gonti's Machinations: the bare AFLifeLost token (no SVar), the sum
		// the LoseLife just before it in this chain requested (gorge
		// loseLife's StoreSVar), read as the life it took (ExLastLifeLost).
		n = lValue{Kind: vLastLifeLost}
	} else if n, err = lc.amount(ps, "LifeAmount", 1); err != nil {
		return err
	}
	if sa.API == "GainLife" && n.Kind == vConst && lc.face.SVars[ps.str("LifeAmount")] == "Number$0" {
		// Exsanguinate's AFLifeLost: the life the previous LoseLife took.
		n = lValue{Kind: vLastLifeLost}
	}
	lc.emit(lInstr{Op: sa.API, Obj: who, A: n})
	return nil
}

// kickedSplitGain recognizes the one split-target gain shape: Defined$
// TargetedController over an amount that is exactly Targeted$CardManaCost,
// in a two-slot chain whose slots are the kicked/un-kicked pair of a
// conditional target (TargetMin$/Max$ = Count$Kicked.0.1 for the unkicked
// root and Count$Kicked.1.0 for the sub under its Condition$ Kicked guard).
// The returned slots are the target indices each arm must read.
func (lc *lowerCtx) kickedSplitGain(ps *params) (unkicked, kicked int, ok bool) {
	if ps.str("Defined") != "TargetedController" {
		return 0, 0, false
	}
	amount := strings.TrimSpace(ps.str("LifeAmount"))
	if body, found := lc.face.SVars[amount]; found {
		amount = strings.TrimSpace(body)
	}
	if amount != "Targeted$CardManaCost" || len(lc.chainT) != 2 {
		return 0, 0, false
	}
	unkicked, kicked = -1, -1
	for _, t := range lc.chainT {
		tg := lc.ab.Targets[t]
		if kickedConstRange(tg.Min, tg.Max, 0, 1) {
			unkicked = t
		}
		if kickedConstRange(tg.Min, tg.Max, 1, 0) {
			kicked = t
		}
	}
	if unkicked < 0 || kicked < 0 || unkicked == kicked {
		return 0, 0, false
	}
	return unkicked, kicked, true
}

func (lc *lowerCtx) apiLosesGame(sa *cards.SA, ps *params) error {
	// gorge effLosesGame / effWinsGame (effects/dice.go): the Defined$
	// player (the controller when absent) loses / wins. A win is blocked
	// only by an "opponents can't win" replacement (kernel winGame).
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rEachOpp, rTriggeredPlayer:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("%s of a non-player target", sa.API)
		}
	default:
		return refuse("%s Defined$ %q", sa.API, who)
	}
	in := lInstr{Op: "LoseGame", Obj: who}
	if sa.API == "WinsGame" {
		in.C = 1
	}
	lc.emit(in)
	return nil
}

func (lc *lowerCtx) apiSetLife(sa *cards.SA, ps *params) error {
	// gorge effSetLife: the one acting player's life becomes
	// LifeAmount$ (Redistribute$ and several players refuse)
	if _, ok := ps.get("Redistribute"); ok {
		return refuse("SetLife Redistribute$")
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rTriggeredPlayer, rTargetController, rEachPlayer, rEachOpp:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 || !t.Max.isConst() || t.Max.N > 1 {
			return refuse("SetLife of target slot %d", who.N)
		}
	default:
		return refuse("SetLife of %s", who)
	}
	n, err := lc.amount(ps, "LifeAmount", 0)
	if err != nil {
		return err
	}
	if _, ok := ps.m["LifeAmount"]; !ok {
		return refuse("SetLife without LifeAmount$")
	}
	lc.emit(lInstr{Op: "SetLife", Obj: who, A: n})
	return nil
}

func (lc *lowerCtx) apiExtort(sa *cards.SA, ps *params) error {
	// CR 702.101a: the printed K:Extort expansion's body, and the
	// latent body a layer-6 AddKeyword$ Extort grant carries
	// (lower_extortgrant.go). The op poses the {W/B} pay/decline
	// election and drains on a pay (dirac/act_extort.go).
	lc.emit(lInstr{Op: "Extort"})
	return nil
}

func (lc *lowerCtx) apiPoison(sa *cards.SA, ps *params) error {
	// gorge effPoison: each Defined$ player gets Num$ poison counters
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rEachOpp, rEachPlayer, rTriggeredPlayer, rDefending, rTargetController, rTriggeredTgtCtrl:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("Poison of a non-player target")
		}
	default:
		return refuse("Poison of %s", who)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Poison", Obj: who, A: n})
	return nil
}

func (lc *lowerCtx) apiRadiation(sa *cards.SA, ps *params) error {
	// gorge effRadiation: each Defined$ player gets Num$ rad counters (a
	// PlayerCounterChange "RAD"); the precombat-main drain they feed is the
	// kernel's own source-less trigger (dirac/act_radiation.go)
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rEachOpp, rEachPlayer, rTriggeredPlayer, rDefending, rTargetController:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("Radiation of a non-player target")
		}
	default:
		return refuse("Radiation of %s", who)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Radiation", Obj: who, A: n})
	return nil
}

func (lc *lowerCtx) apiBlight(sa *cards.SA, ps *params) error {
	// gorge effBlight (CR 701.68): each Defined$ player chooses a
	// creature they control and puts Num$ -1/-1 counters on it; the
	// ValidTgts$ fallback makes a targeted opponent the blighter.
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rEachOpp, rTriggeredPlayer, rDefending, rTargetController:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("Blight of a non-player target")
		}
	default:
		return refuse("Blight of %s", who)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Blight", Obj: who, A: n})
	return nil
}

// followsLoseLife reports whether the instruction emitted last is a LoseLife:
// the GainLife that reads AFLifeLost then follows the loss it names.
func (lc *lowerCtx) followsLoseLife() bool {
	n := len(lc.ab.Prog)
	return n > 0 && lc.ab.Prog[n-1].Op == "LoseLife"
}
