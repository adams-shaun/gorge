package kgen

import (
	"regexp"
	"strconv"
)

// Resolution cost tokens the kernel pays as one atomic election:
//
//   - Ward—AddCounterYou<N/POISON>: the ward's payer gets N poison counters
//     (gorge ward.go beginWardPayment's AddCounterYou arm, offered always).
//   - UnlessCost$ DamageYou<X>: the resolving source deals X damage to the
//     payer (gorge payUnlessDamageCost); X is the face's SVar:X.
//   - UnlessCost$ Draw<X/You>: the payer draws X cards, X the face's SVar:X.
//
// gorge folds an unless X only when it resolves above zero; any other X stays
// an unpriceable token, so the ask is still posed with the decline alone (the
// kernel's canPayGen, amount > 0).

// wardPoisonRe is a ward cost of N poison counters.
var wardPoisonRe = regexp.MustCompile(`^AddCounterYou<([0-9]+)/POISON>$`)

// resUnlessTokenRe is an UnlessCost$ whose amount is the face's SVar:X.
var resUnlessTokenRe = regexp.MustCompile(`^(DamageYou<X>|Draw<X/You>)$`)

// resUnlessDiscardRe is an UnlessCost$ Discard<Name/Card> whose count names a
// face SVar (Court of Ambition's Discard<Y/Card>, Y = Count$Monarch.2.1).
var resUnlessDiscardRe = regexp.MustCompile(`^Discard<([A-Za-z][A-Za-z0-9]*)/Card>$`)

// resUnlessDiscard lowers an UnlessCost$ Discard<Name/Card> of a non-delayed
// body to the payer's discard of the SVar's count (kernel PayDiscard, its
// amount evaluated as the ask is posed). gorge's UnlessCostResolved folds the
// same SVar body into the token at that moment (engine/effects/unless.go
// discardSVarToken); a count resolving to zero stays raw there, the
// unpriceable decline the kernel's amount > 0 gate keeps.
func (lc *lowerCtx) resUnlessDiscard(uc string) (lInstr, bool, error) {
	m := resUnlessDiscardRe.FindStringSubmatch(uc)
	if m == nil {
		return lInstr{}, false, nil
	}
	if _, ok := lc.face.SVars[m[1]]; !ok {
		return lInstr{}, false, nil // no SVar: the cost token refusal stands
	}
	if lc.ab.Kind == akLatent {
		return lInstr{}, true, refuse("unless cost %q in a delayed body", uc)
	}
	v, err := lc.value(m[1])
	if err != nil {
		return lInstr{}, true, err
	}
	return lInstr{CS: "Discard", A: v}, true, nil
}

// wardPoison lowers a ward whose cost is AddCounterYou<N/POISON>.
func (lc *lowerCtx) wardPoison(uc string) (lInstr, bool, error) {
	m := wardPoisonRe.FindStringSubmatch(uc)
	if m == nil {
		return lInstr{}, false, nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 || n > 255 {
		return lInstr{}, true, refuse("ward poison cost %q", uc)
	}
	return lInstr{Op: "Ward", CS: "Poison", A: cst(n), Obj: lRef{Kind: rTriggered}}, true, nil
}

// resUnlessToken lowers an UnlessCost$ DamageYou<X> or Draw<X/You> of a
// non-delayed body to its pay instruction.
func (lc *lowerCtx) resUnlessToken(uc string) (lInstr, bool, error) {
	m := resUnlessTokenRe.FindStringSubmatch(uc)
	if m == nil {
		return lInstr{}, false, nil
	}
	if lc.ab.Kind == akLatent {
		return lInstr{}, true, refuse("unless cost %q in a delayed body", uc)
	}
	if _, ok := lc.face.SVars["X"]; !ok {
		return lInstr{}, true, refuse("unless cost %q without SVar:X", uc)
	}
	v, err := lc.value("X")
	if err != nil {
		return lInstr{}, true, err
	}
	cs := "Draw"
	if m[1] == "DamageYou<X>" {
		cs = "DamageYou"
	}
	return lInstr{CS: cs, A: v}, true, nil
}
