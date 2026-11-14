package kgen

import (
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
	apiOp("Blight", (*lowerCtx).apiBlight)
}

func (lc *lowerCtx) apiGainLife(sa *cards.SA, ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "LifeAmount", 1)
	if err != nil {
		return err
	}
	if sa.API == "GainLife" && n.Kind == vConst && lc.face.SVars[ps.str("LifeAmount")] == "Number$0" {
		// Exsanguinate's AFLifeLost: the life the previous LoseLife took.
		n = lValue{Kind: vLastLifeLost}
	}
	lc.emit(lInstr{Op: sa.API, Obj: who, A: n})
	return nil
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
	case rYou, rEachOpp, rEachPlayer, rTriggeredPlayer, rDefending, rTargetController:
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
