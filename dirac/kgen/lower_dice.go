package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// dieRange is one ResultSubAbilities$ entry (gorge parseDieRanges).
type dieRange struct {
	lo, hi int
	name   string
	isElse bool
}

// rollDice lowers api:RollDice for one die (gorge effRollDice): Sides$
// (default 6) and Modifier$ constant, Amount$ 1. OpRollDie draws the die;
// ResultSVar$ names the result for the later amounts of the ability
// (vRollResult); ResultSubAbilities$ lowers to one SkipIf-guarded block per
// range, in the table's order, before the roll's own SubAbility$ (gorge
// resolves the range's sub inside effRollDice). Ranges must not overlap
// (gorge takes the first match); an Else entry lowers as the one interval
// of results [1+mod, sides+mod] no range covers, refused when that is not
// one interval. Anything else (several dice, IgnoreLower$, ChosenSVar$,
// UseDifferenceBetweenRolls$, Defined$, ...) is unread and refuses.
func (lc *lowerCtx) rollDice(ps *params) error {
	sides, mod := 6, 0
	if v, ok := ps.get("Sides"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 2 || n > 100 {
			return refuse("RollDice Sides$ %q", v)
		}
		sides = n
	}
	if v, ok := ps.get("Amount"); ok && strings.TrimSpace(v) != "1" {
		return refuse("RollDice Amount$ %q", v)
	}
	if v, ok := ps.get("Modifier"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 || n > 50 {
			return refuse("RollDice Modifier$ %q", v)
		}
		mod = n
	}
	var ranges []dieRange
	if v, ok := ps.get("ResultSubAbilities"); ok {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			spec, name, ok := strings.Cut(part, ":")
			if !ok || strings.TrimSpace(name) == "" {
				return refuse("RollDice range %q", part)
			}
			spec, name = strings.TrimSpace(spec), strings.TrimSpace(name)
			if strings.EqualFold(spec, "Else") {
				ranges = append(ranges, dieRange{name: name, isElse: true})
				continue
			}
			loS, hiS, _ := strings.Cut(spec, "-")
			if hiS == "" {
				hiS = loS
			}
			lo, err1 := strconv.Atoi(strings.TrimSpace(loS))
			hi, err2 := strconv.Atoi(strings.TrimSpace(hiS))
			if err1 != nil || err2 != nil || lo > hi {
				return refuse("RollDice range %q", part)
			}
			ranges = append(ranges, dieRange{lo: lo, hi: hi, name: name})
		}
		// first match wins in gorge: refuse an overlap; place Else
		var covered [256]bool
		for _, r := range ranges {
			if r.isElse {
				continue
			}
			for x := max(r.lo, 0); x <= min(r.hi, 255); x++ {
				if covered[x] {
					return refuse("RollDice overlapping ranges")
				}
				covered[x] = true
			}
		}
		for i := range ranges {
			if !ranges[i].isElse {
				continue
			}
			lo, hi := -1, -1
			for x := 1 + mod; x <= sides+mod; x++ {
				if covered[x] {
					continue
				}
				if lo < 0 {
					lo = x
				} else if hi != x-1 {
					return refuse("RollDice Else range is not one interval")
				}
				hi = x
			}
			ranges[i].lo, ranges[i].hi = lo, hi // lo < 0: never matches
		}
	}
	if name := ps.str("ResultSVar"); name != "" {
		if _, ok := lc.face.SVars[name]; ok {
			return refuse("RollDice ResultSVar$ %q shadowed by a card SVar", name)
		}
		lc.rollName, lc.rollAb = name, lc.ab
	}
	lc.emit(lInstr{Op: "RollDie", A: cst(sides), B: cst(mod)})
	roll := lValue{Kind: vRollResult}
	for _, r := range ranges {
		if r.lo < 0 {
			continue // an Else no result reaches
		}
		sa := cards.ResolveSVar(lc.face.SVars, r.name)
		if sa == nil {
			return refuse("RollDice range sub %q", r.name)
		}
		at := len(lc.ab.Prog)
		// SkipIf skips its block unless the condition holds: lo <= roll <= hi
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: roll, Op: opGE, B: cst(r.lo)}})
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: roll, Op: opLE, B: cst(r.hi)}})
		saved := lc.chainT
		lc.chainT = append([]int(nil), lc.chainT...)
		if err := lc.chain(sa, nil); err != nil {
			return err
		}
		lc.chainT = saved
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
		lc.ab.Prog[at+1].Skip = len(lc.ab.Prog) - at - 2
	}
	return nil
}

// flipCoin lowers api:FlipCoin for one flip by the controller (gorge
// effFlipCoin: h.Rand(2) == 0 is heads, a win). The flip is OpRollDie with
// two sides (the same single rng IntN(2) draw): result 1 = win/heads, 2 =
// lose/tails. WinSubAbility$/HeadsSubAbility$ and LoseSubAbility$/
// TailsSubAbility$ lower as blocks guarded by ExRollResult == 1 / 2, in that order,
// before the node's SubAbility$ (gorge resolves the branch inside the
// effect). NoCall$ True only defers the branches past the flip loop, which
// for one flip is the same order. Refused: Amount$, Flipper$/Defined$,
// ForEachPlayer$, FlipUntilYouLose$, the Remember* forms, a flip with its
// own targets, and a branch that reads X (gorge's heads tally), takes
// targets (a named branch is not linked into the cast's targets) or flips
// or rolls again (it would overwrite the result the next block reads).
func (lc *lowerCtx) flipCoin(ps *params) error {
	ps.consume("NoCall", "LockInText")
	branch := func(keys ...string) (string, error) {
		name := ""
		for _, k := range keys {
			if v := strings.TrimSpace(ps.str(k)); v != "" {
				if name != "" {
					return "", refuse("FlipCoin %s$ and another branch", k)
				}
				name = v
			}
		}
		return name, nil
	}
	win, err := branch("WinSubAbility", "HeadsSubAbility")
	if err != nil {
		return err
	}
	lose, err := branch("LoseSubAbility", "TailsSubAbility")
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "RollDie", A: cst(2), B: cst(0), C: int(kop.RollDieCoin), Note: "flip a coin: 1 heads (win), 2 tails"})
	roll := lValue{Kind: vRollResult}
	for i, name := range []string{win, lose} {
		if name == "" {
			continue
		}
		sa := cards.ResolveSVar(lc.face.SVars, name)
		if sa == nil {
			return refuse("FlipCoin branch %q", name)
		}
		for n := sa; n != nil; n = n.Sub {
			if n.API == "FlipCoin" || n.API == "RollDice" {
				return refuse("FlipCoin branch %q flips or rolls again", name)
			}
			if _, ok := n.Params["ValidTgts"]; ok {
				return refuse("FlipCoin branch %q takes targets", name)
			}
			for _, v := range n.Params {
				if lineReadsX(v) {
					return refuse("FlipCoin branch %q reads X", name)
				}
			}
		}
		// SkipIf skips the block unless its condition holds
		at := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: roll, Op: opEQ, B: cst(i + 1)}})
		saved := lc.chainT
		lc.chainT = append([]int(nil), lc.chainT...)
		if err := lc.chain(sa, nil); err != nil {
			return err
		}
		lc.chainT = saved
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	}
	return nil
}
