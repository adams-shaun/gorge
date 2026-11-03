package kgen

import "strings"

// A card with two or more mana abilities (kmana2). The kernel's mana
// source model is one unit per permanent, read from the card's first mana
// ability (kernel prep.go manaAb), so a further mana ability is played only
// where the kernel models it:
//
//   - every mana ability is a plain "{T}: Add {c}" of one fixed colour (a
//     dual land's or a triome's intrinsic abilities, CR 305.6): gorge's
//     planner census lists the source once with one alternative per
//     ability (rules windowManaUnitsWith), which the kernel models exactly
//     as one deterministic unit producing the union of the colours
//     (kernel mana_multicolour.go, pcard.manaMulti). The abilities must be in
//     W U B R G C order with distinct colours: the payment window activates
//     a multi-ability source's first ability (the oracle's payWindowAnswer
//     takes the first option of gorge's "choose a mana ability" ask, whose
//     fixed options carry no mana symbol), the lowest colour of the union;
//   - the first is a plain unrestricted "{T}: Add ..." and each other is a
//     census gap the kernel models or the frozen lockstep pools prove: a
//     restricted "{T}: Add ..." (Secluded Courtyard's any-colour one,
//     genPrep.anyR; the Villages' creature-only colour) or a "{N}, {T}:
//     Add one mana of any color" (Hidden Grotto's filter, genPrep.filt;
//     Three Tree City's dynamic amount). gorge's census excludes both
//     (RestrictValid; a cost beyond {T}), so its planner pays from the
//     first ability alone, as the kernel does.
//
// Every other shape (a second ability with a sacrifice, life, counter or
// tap-another cost: a last-resort alternative gorge's planner may take; an
// unrestricted choice beside a fixed one; a paid fixed-colour or hybrid
// filter; a dynamic fixed colour) is refused.
func (lc *lowerCtx) refuseMultiMana() {
	var ms []*lAbility
	for i := range lc.out.Abilities {
		if a := lc.out.Abilities[i]; a.Kind == akMana && !a.Latent && a.Mana != nil {
			ms = append(ms, a)
		}
	}
	for _, a := range lc.out.Abilities {
		if a.Kind == akMana && a.Latent && a.ActCond.Kind != cNone {
			lc.refuseNode("card", "gated granted mana ability")
			return
		}
	}
	if lc.refuseGatedMana(ms) || lc.refuseReflectedMana(ms) {
		return
	}
	if len(ms) < 2 {
		return
	}
	if multiFixedMana(ms) {
		return
	}
	if len(ms) == 2 && plainTapMana(ms[0], false) && unrestrictedOne(ms[0]) && ms[0].Mana.Colors == "C" && painTapMana(ms[1]) {
		// a painland (Adarkar Wastes): {T}: Add {C}, then {T}: Add one of
		// two colours with a "deals N damage to you" rider -- gorge's
		// normal C alternative beside last-resort coloured ones
		// (pay.PaymentPlanDamageRider; kernel mana_pain.go)
		return
	}
	if len(ms) == 2 && plainTapMana(ms[0], false) && unrestrictedOne(ms[0]) && ms[0].Mana.Colors == "C" && lifeTapMana(ms[1]) {
		// {T}: Add {C}, then {T}, Pay N life: Add one mana of any colour
		// (Starting Town): gorge's last-resort Consequence{Life: N}, priced
		// and guarded as a painland's damage (kernel painLife)
		return
	}
	if len(ms) > 2 || !plainTapMana(ms[0], false) || !unrestrictedOne(ms[0]) {
		lc.refuseNode("card", "%d mana abilities: not a fixed-colour set, nor a plain {T} ability beside one modelled gap", len(ms))
		return
	}
	for i, a := range ms[1:] {
		if !manaGapModelled(a) {
			lc.refuseNode("card", "%d mana abilities: mana ability %d (cost %v, produces %s) is not modelled beside the first", len(ms), i+1, a.Cost.Mana, a.Mana.Colors)
			return
		}
	}
}

// multiFixedMana: every ability of ms is a plain {T} ability of one fixed
// colour, in W U B R G C order, colours distinct.
func multiFixedMana(ms []*lAbility) bool {
	last := -1
	for _, a := range ms {
		if !plainTapMana(a, false) || !unrestrictedOne(a) || a.Mana.Choice || len(a.Mana.Colors) != 1 {
			return false
		}
		c := strings.Index("WUBRGC", a.Mana.Colors)
		if c <= last {
			return false
		}
		last = c
	}
	return true
}

// plainTapMana: a costs exactly {T} (plus generic mana when paid), has no
// rider, condition, limit or gate, and is unrestricted with amount 1.
func plainTapMana(a *lAbility, paid bool) bool {
	c := &a.Cost
	m := &c.Mana
	if !c.Tap || c.SacSelf || c.Sac != nil || c.DiscardN != 0 || c.DiscardSelf || c.Loyalty != 0 || c.SubCtr != 0 || c.PayLife != 0 ||
		c.TapN != 0 || c.TapHost || c.SacHost || c.Forage != nil || c.ExileG != nil || c.Energy != 0 || c.ReturnSelf || c.ExileSelf || c.Return != nil {
		return false
	}
	if m.Pips != [5]int{} || len(m.Hybrid) > 0 || m.X || m.C != 0 || (m.Generic != 0) != paid {
		return false
	}
	if len(a.Prog) > 0 || a.ActCond.Kind != cNone || a.Limit != 0 || a.TurnOnce || a.Gate != nil || a.Sorcery || a.Loyalty {
		return false
	}
	mn := a.Mana
	if mn.Special != "" || mn.SpecialF != nil || mn.RestrictAb != nil && mn.Restrict == nil || mn.Colors == "" || mn.Colors == "Chosen" {
		return false
	}
	return true
}

// manaGapModelled: a, a mana ability beside a plain first one, is a census
// gap the kernel models or the frozen pools prove (refuseMultiMana).
func manaGapModelled(a *lAbility) bool {
	switch {
	case plainTapMana(a, false):
		// a restricted {T} ability, amount 1 (Courtyard, whose restriction
		// also names activated abilities; the Villages)
		// of one colour or any colour (the kernel's anyR branches on all
		// five: Tournament Grounds' {R}, {W} or {B} is not modelled)
		return a.Mana.Restrict != nil && a.Mana.Amount.Kind == vConst && a.Mana.Amount.N == 1 &&
			(len(a.Mana.Colors) == 1 || a.Mana.Colors == "WUBRG")
	case plainTapMana(a, true):
		// {N}, {T}: one mana of any colour, unrestricted (Hidden Grotto;
		// Three Tree City's dynamic amount)
		// (the kernel's filt reads amount 1; a literal amount above it,
		// Baxter Building's {4},{T}: four mana, is not modelled)
		amt := a.Mana.Amount
		return a.Mana.Restrict == nil && a.Mana.RestrictAb == nil && a.Mana.Colors == "WUBRG" && (amt.Kind == vConst && amt.N == 1 || amt.Kind > vConst)
	}
	return false
}

// unrestrictedOne: a's mana is unrestricted, amount 1.
func unrestrictedOne(a *lAbility) bool {
	return a.Mana.Restrict == nil && a.Mana.RestrictAb == nil && a.Mana.Amount.Kind == vConst && a.Mana.Amount.N == 1
}

// refuseGatedMana handles a mana ability with an IsPresent$ gate
// (CR 602.5b; gorge pay.ManaActivationGateHolds): the kernel reads the gate
// only as a planner/window variant of the source (kernel
// mana_gate.go), which models exactly two shapes:
//
//   - the card's only mana ability is gated, plain {T}, amount 1, unrestricted;
//   - two abilities: a plain ungated {T} "Add {c1}" then a gated plain {T}
//     "Add {c2}" (a Verge), c1 != c2, both fixed one-colour, amount 1.
//
// Either colour order: the payment window activates ability 0 (kernel
// mana_gate.go). A {C} ability beside a coloured one is refused.
// It reports whether ms holds a gate (the caller then has nothing to add).
func (lc *lowerCtx) refuseGatedMana(ms []*lAbility) bool {
	gated := 0
	for _, a := range ms {
		if a.ActCond.Kind != cNone {
			gated++
		}
	}
	if gated == 0 {
		return false
	}
	// the gate may be the only thing off the plain shape
	plain := func(a *lAbility) bool {
		c := *a
		c.ActCond = lCond{}
		return plainTapMana(&c, false) && unrestrictedOne(&c)
	}
	for _, a := range ms {
		if !plain(a) {
			lc.refuseNode("card", "gated mana ability beside a mana ability or cost not modelled (plain {T}, amount 1, unrestricted only)")
			return true
		}
		if a.ActCond.Kind != cNone && a.ActCond.Kind != cPresent {
			lc.refuseNode("card", "gated mana ability with a non-IsPresent gate")
			return true
		}
	}
	switch {
	case len(ms) == 1:
	case len(ms) > 2:
		lc.refuseNode("card", "gated mana ability among %d mana abilities", len(ms))
	case ms[0].ActCond.Kind != cNone:
		lc.refuseNode("card", "gate on the first of two mana abilities")
	case ms[0].Mana.Colors == "C" && !ms[0].Mana.Choice && gatedColourChoice(ms[1]):
		// {T}: Add {C}, then a gated {T}: Add one mana of any colour
		// (Mirrex) or of a Combo of two or more distinct colours
		// (the MSH basic-land cycle): gorge's census lists
		// one unit, the {C} alternative then one per colour; the kernel's
		// gate-on view is the union C|colours, the gate-off view {C} alone
		// (prepManaGate, gatedChoiceUnion)
	case len(ms[0].Mana.Colors) != 1 || ms[0].Mana.Choice || len(ms[1].Mana.Colors) != 1 || ms[1].Mana.Choice:
		lc.refuseNode("card", "gated mana ability beside a choice ability")
	case ms[0].Mana.Colors == ms[1].Mana.Colors:
		lc.refuseNode("card", "gated mana ability of the same colour as the first")
	case strings.Index("WUBRG", ms[0].Mana.Colors) < 0 || strings.Index("WUBRG", ms[1].Mana.Colors) < 0:
		lc.refuseNode("card", "gated mana ability producing {C} beside another")
	}
	return true
}

// gatedColourChoice reports whether a is "any colour" or a Combo choice among
// two or more distinct WUBRG colours.
func gatedColourChoice(a *lAbility) bool {
	c := a.Mana.Colors
	if c == "WUBRG" {
		return true
	}
	if !a.Mana.Choice || len(c) < 2 {
		return false
	}
	for i := 0; i < len(c); i++ {
		if strings.IndexByte("WUBRG", c[i]) < 0 || strings.IndexByte(c[i+1:], c[i]) >= 0 {
			return false
		}
	}
	return true
}

// painRider reports whether a's program is exactly a painland's rider: its
// source deals a constant N >= 1 damage to its controller ("CARDNAME deals
// 1 damage to you", gorge pay.PaymentPlanDamageRider).
func painRider(a *lAbility) bool {
	if a.Kind != akMana || len(a.Prog) != 1 {
		return false
	}
	in := &a.Prog[0]
	return in.Op == "DealDamage" && in.Obj.Kind == rYou && in.A.isConst() && in.A.N >= 1 && in.F == nil && in.Cond.Kind == cNone && in.CS == "" && in.Skip == 0
}

// lifeTapMana: a {T}, Pay N life mana ability (amount 1, unrestricted, no
// program) of one colour, a choice of two, or any colour.
func lifeTapMana(a *lAbility) bool {
	if a.Kind != akMana || len(a.Prog) != 0 || a.Cost.PayLife < 1 {
		return false
	}
	c := *a
	c.Cost.PayLife = 0
	if !plainTapMana(&c, false) || !unrestrictedOne(&c) {
		return false
	}
	switch n := len(c.Mana.Colors); {
	case n == 1:
		return strings.Contains("WUBRG", c.Mana.Colors)
	case n == 2:
		return c.Mana.Choice && strings.Trim(c.Mana.Colors, "WUBRG") == ""
	case n == 5:
		return c.Mana.Colors == "WUBRG" // Produced$ Any
	}
	return false
}

// painTapMana: a plain {T} mana ability (amount 1, unrestricted) of one
// colour or a choice of two, carrying a painland rider (painRider).
func painTapMana(a *lAbility) bool {
	if !painRider(a) {
		return false
	}
	c := *a
	c.Prog = nil
	if !plainTapMana(&c, false) || !unrestrictedOne(&c) {
		return false
	}
	switch n := len(c.Mana.Colors); {
	case n == 1:
		return strings.Contains("WUBRG", c.Mana.Colors)
	case n == 2:
		return c.Mana.Choice && strings.Contains("WUBRG", c.Mana.Colors[:1]) && strings.Contains("WUBRG", c.Mana.Colors[1:])
	}
	return false
}
