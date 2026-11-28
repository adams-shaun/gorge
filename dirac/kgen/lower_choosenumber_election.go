package kgen

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The secret ChooseNumber election (gorge effects.effChooseNumberElection,
// choose.go): api:ChooseNumber with MatchedAbility$ / UnmatchedAbility$ and
// Defined$ TargetedAndYou (Expert-Level Safe, the one corpus carrier). gorge
// asks the controller, then the targeted opponent (repeatPlayers'
// AliveFrom(controller) order), each a KChoose over chooseNumberAsk's
// Min$..Max$ list, posts no per-chooser event before the last answer, then
// records the controller's pick on the source, reveals both and resolves the
// matched or the unmatched body.
//
// One OpExt ExtChooseNumberElection does the asks (Obj the targeted opponent,
// B the Max$ value, C the Min$ floor with ChooseNumberMid's has-Max flag),
// keeps the first pick in resolution scratch until the second is answered,
// and leaves the verdict in the resolution's result slot (ExRollResult, the
// one Clash's and FlipCoin's branches read): 1 matched, 2 not. The two bodies
// lower as SkipIf-guarded blocks, MatchedAbility$ then UnmatchedAbility$, as
// lower_clash.go's do.
//
// Refused: any other Defined$ (a single chooser, or a set the kernel would
// order differently), a target that is not exactly ValidTgts$ Opponent (the
// dedupe could merge the two choosers), non-literal bounds, Random$, the
// Guesser$ shape (The Toymaker's Trap) and a branch that would overwrite the
// verdict, take targets or read X.
func (lc *lowerCtx) chooseNumberElection(ps *params) error {
	if d := ps.str("Defined"); d != "TargetedAndYou" {
		return refuse("ChooseNumber election Defined$ %q", d)
	}
	if !ps.is("Secretly", "True") {
		return refuse("ChooseNumber election without Secretly$ True")
	}
	if _, ok := ps.get("Random"); ok {
		return refuse("ChooseNumber election Random$")
	}
	who, err := lc.defined("Targeted")
	if err != nil {
		return refuse("ChooseNumber secret election without a targeted opponent: %v", err)
	}
	if who.Kind != rTarget {
		return refuse("ChooseNumber secret election chooser %q is not a target", who)
	}
	if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players != plOpp || len(t.F.Clauses) != 0 {
		return refuse("ChooseNumber secret election target is not exactly an opponent")
	}
	lit := func(k string) (int, error) {
		n, err := strconv.Atoi(ps.str(k))
		if err != nil || n < 0 || n > numberKernelCeiling {
			return 0, refuse("ChooseNumber election %s$ %q is not a small literal", k, ps.str(k))
		}
		return n, nil
	}
	lo, err := lit("Min")
	if err != nil {
		return err
	}
	hi, err := lit("Max")
	if err != nil {
		return err
	}
	if lo >= hi {
		return refuse("ChooseNumber election Min$ %d Max$ %d offers no choice", lo, hi)
	}
	matched, unmatched := clashBranch(ps, "MatchedAbility"), clashBranch(ps, "UnmatchedAbility")
	ps.consume("Defined", "Secretly", "Min", "Max", "AILogic", "ListTitle")
	lc.emit(lInstr{Op: "ChooseNumberElection", Obj: who, B: cst(hi), C: lo | 0x100,
		Note: "secret number election: result 1 matched, 2 not"})
	return lc.electionBranches(matched, unmatched)
}

// electionBranches lowers the matched then the unmatched body, each a block
// guarded by the result slot (1 matched, 2 not).
func (lc *lowerCtx) electionBranches(names ...string) error {
	result := lValue{Kind: vRollResult}
	for i, name := range names {
		if name == "" {
			continue
		}
		sa := cards.ResolveSVar(lc.face.SVars, name)
		if sa == nil {
			return refuse("ChooseNumber election branch %q", name)
		}
		if err := electionBranchShape(name, sa); err != nil {
			return err
		}
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

// electionBranchShape refuses a branch body that would overwrite the
// verdict the next block reads, take targets or read X.
func electionBranchShape(name string, sa *cards.SA) error {
	for n := sa; n != nil; n = n.Sub {
		switch n.API {
		case "FlipCoin", "RollDice", "Clash", "ChooseNumber":
			return refuse("ChooseNumber election branch %q flips, rolls, clashes or chooses again", name)
		}
		if _, ok := n.Params["ValidTgts"]; ok {
			return refuse("ChooseNumber election branch %q takes targets", name)
		}
		for _, v := range n.Params {
			if lineReadsX(v) {
				return refuse("ChooseNumber election branch %q reads X", name)
			}
		}
	}
	return nil
}

// isChooseNumberElection reports whether a ChooseNumber node is the
// matched/unmatched election rather than a single chooser's ask.
func isChooseNumberElection(ps *params) bool {
	_, m := ps.get("MatchedAbility")
	_, u := ps.get("UnmatchedAbility")
	_, g := ps.get("Guesser")
	return (m || u) && !g
}
