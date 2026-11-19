package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// api:Clash (CR 701.31; gorge effClash, effects/clash.go). The resolving
// controller clashes with an opponent they choose: both reveal their top card, then
// each (the controller first, then the opponent, APNAP order) puts their own
// card on the top or bottom of their library, and the strictly higher mana
// value wins (a tie, or two empty libraries, has no winner).
//
// One OpExt ExtClash does all of it and leaves the verdict in the
// resolution's result slot (ExRollResult, the one FlipCoin's branches read):
// 1 the controller won, 2 anything else. WinSubAbility$ lowers as a block
// guarded by result == 1 and OtherwiseSubAbility$ by result == 2, in that
// order and before the node's own SubAbility$, as gorge resolves its branch
// inside the effect. The branch is refused when it would overwrite the
// verdict (a flip, a roll or another clash), takes targets (a named branch is
// not linked into the cast's targets) or reads X.
//
// Refused: Defined$, ValidTgts$ and RememberClasher$ (the clash opponent is
// the controller's chosen one, asked past two seats, seat-exact; a defined, targeted or
// remembered one is not lowered)
// and every other parameter, as an unread key.
func (lc *lowerCtx) clash(ps *params) error {
	// the clash opponent: gorge's controller chooses one living opponent
	// (CR 701.31a, chooseClashOpponent), an ask only at three or more
	// seats, which the kernel poses too (act_clash.go raPodClashOpp) and
	// holds in the resolution's chosen-player byte while the placements are
	// asked: a ChoosePlayer of this ability would share it
	if lc.ab != nil && (lc.chosenMidAb == lc.ab || lc.chosenResAb == lc.ab || lc.chosenPlAb == lc.ab || lc.chosenOppAb == lc.ab) {
		return refuse("Clash beside a ChoosePlayer of its ability")
	}
	win, other := clashBranch(ps, "WinSubAbility"), clashBranch(ps, "OtherwiseSubAbility")
	lc.emit(lInstr{Op: "Clash", Note: "clash with the opponent: result 1 won, 2 not"})
	result := lValue{Kind: vRollResult}
	for i, name := range []string{win, other} {
		if name == "" {
			continue
		}
		sa := cards.ResolveSVar(lc.face.SVars, name)
		if sa == nil {
			return refuse("Clash branch %q", name)
		}
		for n := sa; n != nil; n = n.Sub {
			switch n.API {
			case "FlipCoin", "RollDice", "Clash":
				return refuse("Clash branch %q flips, rolls or clashes again", name)
			}
			if _, ok := n.Params["ValidTgts"]; ok {
				return refuse("Clash branch %q takes targets", name)
			}
			for _, v := range n.Params {
				if lineReadsX(v) {
					return refuse("Clash branch %q reads X", name)
				}
			}
		}
		// SkipIf skips the block unless its condition holds
		at := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: result, Op: opEQ, B: cst(i + 1)}})
		saved := lc.chainT
		lc.chainT = append([]int(nil), lc.chainT...)
		err := lc.chain(sa, nil)
		lc.chainT = saved
		if err != nil {
			return err
		}
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	}
	return nil
}

// clashBranch reads and consumes a branch SVar name, "" when absent.
func clashBranch(ps *params, key string) string {
	name := strings.TrimSpace(ps.str(key))
	if name != "" {
		ps.consume(key)
	}
	return name
}

// buildClash is OpExt ExtClash.
func buildClash(ki *kop.Instr) {
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtClash))
}
