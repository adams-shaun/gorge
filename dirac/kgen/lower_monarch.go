package kgen

import "strings"

// The monarch (CR 725): api BecomeMonarch (gorge effects/game.go
// effBecomeMonarch) lowers as OpExt ExtBecomeMonarch over the player who
// becomes it; the designation's reads are Count$Monarch.<yes>.<no>
// (vMonarch N 0, value.go), CheckDefinedPlayer$ You.isMonarch and
// Opponent.isMonarch (vMonarch N 0 / kop.MonarchOpp, lower_trigger.go) and
// Mode$ BecomeMonarch (becomeMonarchTrigger). The kernel's half is
// dirac/act_monarch.go.

// becomeMonarch lowers BecomeMonarch: the resolving controller (no Defined$,
// or Defined$ You), a player target (ValidTgts$ Player / Opponent: Éomer) or
// the trigger's player becomes the monarch. Any other Defined$ refuses.
func (lc *lowerCtx) becomeMonarch(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rTriggeredPlayer:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("BecomeMonarch of a non-player target")
		}
	default:
		return refuse("BecomeMonarch Defined$ %q", ps.str("Defined"))
	}
	lc.emit(lInstr{Op: "BecomeMonarch", Obj: who, Note: "the player becomes the monarch (gorge effBecomeMonarch)"})
	return nil
}

// becomeMonarchTrigger lowers Mode$ BecomeMonarch (gorge
// trigmatch.BecomeMonarchMatches): a player (ValidPlayer$ You / Opponent /
// any) becomes the monarch, the new monarch being the trigger's player.
// BeginTurn$ You is the intervening-if "if you were the monarch as the turn
// began" (kop.TrigNMonarchBeganTurn); any other value fails closed in gorge
// and refuses here.
func (lc *lowerCtx) becomeMonarchTrigger(tp *params, tr *lTrigger) error {
	if tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("BecomeMonarch trigger outside the battlefield")
	}
	w, ok := whoOfPlayer(tp.str("ValidPlayer"))
	if !ok {
		return refuse("BecomeMonarch ValidPlayer %q", tp.str("ValidPlayer"))
	}
	tr.Event, tr.Whose = "BecomeMonarch", w
	if bt, ok := tp.get("BeginTurn"); ok {
		if strings.TrimSpace(bt) != "You" {
			return refuse("BecomeMonarch BeginTurn$ %q", bt)
		}
		tr.N = 1 // kop.TrigNMonarchBeganTurn
	}
	return nil
}

// monarchCheck is CheckDefinedPlayer$ <You|Opponent>.isMonarch as an
// intervening-if (gorge checkDefinedPlayerHolds' isMonarch property): the
// controller, or an opponent still in the game, is the monarch.
func monarchCheck(cdp string) (lCond, bool) {
	var n int
	switch cdp {
	case "You.isMonarch":
	case "Opponent.isMonarch":
		n = 1 // kop.MonarchOpp
	default:
		return lCond{}, false
	}
	return lCond{Kind: cValue, A: lValue{Kind: vMonarch, N: n, Sub: []lValue{cst(1), cst(0)}}, Op: opGE, B: cst(1)}, true
}

// monarchEffectTrigger refuses an Effect's Mode$ BecomeMonarch trigger by
// name: Palace Jailer's "exile target creature an opponent controls until an
// opponent becomes the monarch" (CR 610.3). gorge registers the trigger as a
// static delayed promise that returns the card when an opponent of the
// Effect's controller becomes the monarch, but its exile does not read the
// promise: a card is exiled even when an opponent became the monarch after the
// ability triggered, which CR 610.3b forbids (gorge's 610.3a/b guard,
// effects exileHostGoneFor, reads only Duration$ UntilHostLeavesPlay). gorge
// does not play the card text yet, so the kernel has nothing to follow.
func monarchEffectTrigger() error {
	return refuse("Effect Mode$ BecomeMonarch trigger: an exile until an opponent becomes the monarch (Palace Jailer) is not modelled; gorge's exile misses CR 610.3b for it")
}
