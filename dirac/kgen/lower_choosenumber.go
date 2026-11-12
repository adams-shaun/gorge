package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The as-enters number choice (CR 614.12): K:ETBReplacement:Other:<SVar>
// with DB$ ChooseNumber | Defined$ You (Nyota Uhura, Communications; Talion,
// the Kindly Lord; Sanctum Prelate). gorge asks it at the entry boundary
// (etbOptions' "number" arm), its options from effects.ETBNumberChoices, and
// keeps the answer in Object.ChosenNumber, which a filter bound "Chosen"
// reads (specCtx's Resolve: cmcEQChosen). The kernel lowers it to the enters
// program's first instruction, OpExt ExtChooseNumber (B the lowest number, C
// the highest), asked as the colour choice is, and keeps the answer in the
// permanent's chosen byte, read by ExChosenNumber.

func init() { exprKinds[vChosenNumber] = kop.ExChosenNumber }

// numberOfferCeiling is gorge's chooseNumberMaxOffer: without a Max$ the ask
// offers Min$ (or 0) to this.
const numberOfferCeiling = 12

// numberKernelCeiling is the largest number the kernel's chosen byte keeps
// apart from its "none" value; no corpus as-enters bound comes near it.
const numberKernelCeiling = 100

// entersChoosingNumber reports whether face f chooses a number as it enters.
func entersChoosingNumber(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.With != nil && r.With.API == "ChooseNumber" && r.ParamStr(cards.PKKeyword) == "ETBReplacement" {
			return true
		}
	}
	return false
}

// etbChooseNumber lowers an ETB replacement's ChooseNumber node: the
// entering permanent's own, unconditional, sole choice, with literal bounds.
// The bounds follow gorge's chooseNumberAsk; a bound gorge cannot honour
// falls back to 0..12 there (ETBNumberChoices), and here too.
func (lc *lowerCtx) etbChooseNumber(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return refuse("ETB ChooseNumber beside another ETB replacement part")
	}
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("ETB ChooseNumber Defined$ %q", d)
	}
	lit := func(k string) (int, bool, error) {
		v, ok := ps.get(k)
		if !ok || strings.TrimSpace(v) == "" {
			return 0, false, nil
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false, refuse("ETB ChooseNumber %s$ %q is not a literal", k, v)
		}
		return n, true, nil
	}
	mn, hasMin, err := lit("Min")
	if err != nil {
		return err
	}
	mx, hasMax, err := lit("Max")
	if err != nil {
		return err
	}
	lo, hi := max(mn, 0), numberOfferCeiling
	switch {
	case !hasMax:
		if hasMin && mn > numberOfferCeiling {
			lo = 0
		}
	case mx < 0 || lo > mx:
		lo = 0
	default:
		hi = mx
	}
	if hi > numberKernelCeiling {
		return refuse("ETB ChooseNumber Max$ %d above %d", hi, numberKernelCeiling)
	}
	lc.emit(lInstr{Op: "ChooseNumber", Obj: lRef{Kind: rYou}, B: cst(lo), C: hi, Note: "choice of a number as it enters"})
	return nil
}

// chooseNumberEffect lowers api:ChooseNumber outside an as-enters replacement
// (gorge effects.effChooseNumber's resolution-time ask: Well of Lost Dreams,
// Yusri, Fortune's Flame, Tibalt's Trickery, the join forces cards and the
// "pay one or more {E}" family). It emits ChooseNumberMid with the chooser,
// the literal Min$ floor and, when the card states one, the Max$ value
// (a literal, an SVar or an inline Count$ expression). Random$ True makes
// the kernel draw the number instead of asking, as cr 107.1 / the Oracle
// text say. The secret election (Secretly$, MatchedAbility$, Guesser$ and
// their kin: Expert-Level Safe, The Toymaker's Trap) is a different question
// -- several players answering secretly then a comparison -- and is kept
// refused with its own reason, not modelled as a single chooser.
func (lc *lowerCtx) chooseNumberEffect(ps *params) error {
	for _, k := range []string{"MatchedAbility", "UnmatchedAbility", "Guesser", "GuessWrong", "GuessCorrect", "RemoveChoices"} {
		if _, ok := ps.get(k); ok {
			return refuse("ChooseNumber %s$: the secret election is not modelled", k)
		}
	}
	if ps.is("Secretly", "True") {
		return refuse("ChooseNumber Secretly$: the secret election is not modelled")
	}
	if _, ok := ps.get("RememberChosen"); ok {
		return refuse("ChooseNumber RememberChosen$: the secret election is not modelled")
	}
	random := ps.is("Random", "True")
	if v, ok := ps.get("Random"); ok && v != "True" && v != "False" {
		return refuse("ChooseNumber Random$ %q", v)
	}
	lo := 0
	if v, ok := ps.get("Min"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return refuse("ChooseNumber Min$ %q is not a literal", v)
		}
		if n > 0 {
			// The kernel keeps the floor in the chosen byte's low 8 bits,
			// beside the has-Max (0x100) and Random (0x200) flags: a larger
			// literal would alias into a flag and change the question. No
			// corpus card states one; refuse by name rather than lower a
			// bound that cannot be kept.
			if n > 0xff {
				return refuse("ChooseNumber Min$ %d above the kernel's 255 floor", n)
			}
			lo = n
		}
	}
	var maxV lValue
	hasMax := false
	if v, ok := ps.get("Max"); ok && strings.TrimSpace(v) != "" {
		mv, err := lc.value(v)
		if err != nil {
			return err
		}
		maxV, hasMax = mv, true
	}
	// Keep the card's stated Min$ even past the default 0..12 list. gorge's
	// chooseNumberAsk fails a Min$-only bound there closed (it records the
	// deterministic 0 with no question), and the kernel's
	// chooseNumberMidBounds reads the same floor and takes the same fallback;
	// zeroing it here would instead ask 0..12, a question gorge never poses.
	who := lRef{Kind: rYou}
	if v, ok := ps.get("Defined"); ok {
		var err error
		if who, err = lc.defined(strings.TrimSpace(v)); err != nil {
			return err
		}
	}
	switch who.Kind {
	case rYou, rRemembered, rEach, rEachPlayer, rEachOpp, rTriggeredPlayer, rTargetController:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("ChooseNumber of a non-player target")
		}
	default:
		return refuse("ChooseNumber Defined$ %q", who)
	}
	ps.consume("ChooseAnyNumber", "ListTitle", "AILogic")
	in := lInstr{Op: "ChooseNumberMid", Obj: who, B: maxV, C: lo, Note: "choice of a number while resolving"}
	if hasMax {
		in.C |= 0x100
	}
	if random {
		in.C |= 0x200
	}
	lc.emit(in)
	return nil
}
