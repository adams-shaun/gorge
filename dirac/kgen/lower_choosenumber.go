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
