package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
//     genPrep.anyR; the Villages' creature-only colour), a "{N}, {T}:
//     Add one mana of any color" (Hidden Grotto's filter, genPrep.filt;
//     Three Tree City's dynamic amount) or "{N}, {T}: Add N mana in any
//     combination of colors" (Baxter Building's combination filter,
//     mana_filter.go filtCombo). gorge's census excludes both (RestrictValid;
//     a cost beyond {T}), so its planner pays from the first ability alone,
//     as the kernel does;
//   - a plain first ability ({C} or a two-colour {T} choice) beside a
//     "{T}, Pay N life" ability (Starting Town, Haunted Screen): gorge's
//     last-resort Consequence{Life: N} beside its normal alternatives,
//     which the kernel models as a pain source (mana_pain.go painLife).
//
// A plain fixed-colour {T} first ability beside "{T}, tap an untapped creature
// you control: Add one mana of any color" (Scene of the Crime) is accepted by
// manaGapModelled (tapXGapMana): the kernel's tap-another gap kind reads the
// card's second ability (mana_tapanother.go tapXAbOf). So is "{T}, remove N
// counters from this: Add <fixed colour>" beside a plain {T} first ability
// (Weather Maker's {C}{C}; ctrTapGapMana): the kernel's counter gap kind
// reads the card's second ability (mana_countercost.go ctrAbOf).
//
// Every other shape (a second ability with a sacrifice or counter cost, or
// another tap-another one (gorge's census leaves a counter or tap-another one a
// relaxable gap its prefix search activates, not a planner alternative); an
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
	ms = slices.DeleteFunc(ms, func(a *lAbility) bool {
		if a == ms[0] || !redundantMana(ms[0], a) {
			return false
		}
		if a.Mana.Amount.Kind == vOppsAttacked {
			lc.twoSeat("the opponents attacked this turn read as at most one")
		}
		return true
	})
	if lc.refuseEachMana(ms) || lc.refuseGatedMana(ms) || lc.refuseReflectedMana(ms) || lc.refuseCtrRiderBeside(ms) {
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
	if len(ms) == 2 && plainTapMana(ms[0], false) && unrestrictedOne(ms[0]) && primaryMana(ms[0]) && lifeTapMana(ms[1]) {
		// {T}: Add {C}, then {T}, Pay N life: Add one mana of any colour
		// (Starting Town), or a plain two-colour {T} choice beside a
		// three-colour life ability (Haunted Screen): gorge's last-resort
		// Consequence{Life: N} beside its normal alternatives, priced and
		// guarded as a painland's damage (kernel painLife)
		return
	}
	if len(ms) == 2 && dynamicLargerSameColour(ms[0], ms[1]) {
		// {T}: Add {G}, then {T}: Add {G} for each creature you control
		// (Itlimoc, Cradle of the Sun): gorge's census unit holds both
		// alternatives (the evaluated one at its count, payment_plan.go's
		// evaluated-amount layer) and the planner takes the best per payment;
		// the kernel's per-count pair view (mana_dynpair.go) plans it
		return
	}
	if len(ms) == 2 && (restrictedFirstPair(ms[0], ms[1]) || restrictedFixedPair(ms[0], ms[1])) {
		// a restricted pair whose two abilities are disjoint by payment class:
		// a spell-only {T} ability of one mana then an activation-only one of
		// several (Steelswarm Operator: {U} for artifact spells, {U}{U} to
		// activate artifact abilities), or a chosen/fixed first ability then
		// an activation-only larger one (Sunken Citadel: chosen 1, chosen 2
		// for abilities of land sources). The kernel plans whichever ability
		// the payment admits (mana_restrictedsource.go prepRestrSrc,
		// restrViewW); the fixed spell-only pair is Tablet of Discovery.
		return
	}
	if len(ms) == 2 && chosenTapMana(ms[0]) && feeEachAmongMana(ms[1]) {
		// Tarnation Vista: a chosen-colour {T} beside a "{N}, {T}: for each colour
		// among ... add one of that colour". gorge's census leaves the fee-paid
		// each-colour ability an unrelaxable gap (its Special production names
		// no colour, so no funding is searched), which the prefix search
		// reaches only with the fee already floating: the activation then asks
		// which ability, and the fee is paid from the pool. The kernel's
		// each-colour search (mana_eachfee.go) mirrors it; the chosen {T} stays
		// the planner's plain unit (pc.manaChosen)
		return
	}
	if len(ms) > 2 && !anyRFiltMana(ms) || !plainTapMana(ms[0], false) || !unrestrictedOne(ms[0]) {
		lc.refuseNode("card", "%d mana abilities: not a fixed-colour set, nor a plain {T} ability beside one modelled gap", len(ms))
		return
	}
	for i, a := range ms[1:] {
		if !manaGapModelled(ms[0], a) {
			lc.refuseNode("card", "%d mana abilities: mana ability %d (cost %v, produces %s) is not modelled beside the first: %s", len(ms), i+1, a.Cost.Mana, a.Mana.Colors, manaGapWhy(ms[0], a))
			return
		}
	}
}

// chosenTapMana: a is a bare "{T}: Add one mana of the chosen color".
func chosenTapMana(a *lAbility) bool {
	if a.Mana.Colors != "Chosen" || a.Mana.Special != "" || a.Mana.Choice || a.Mana.Each || !unrestrictedOne(a) {
		return false
	}
	c := *a
	m := *a.Mana
	m.Colors = "C"
	c.Mana = &m
	return plainTapMana(&c, false)
}

// feeEachAmongMana: a is "{N}, {T}: For each color among <permanents>, add one
// mana of that color" (Tarnation Vista's second ability): a generic fee only.
func feeEachAmongMana(a *lAbility) bool {
	if a.Mana.Special != "EachColorAmong" || a.Mana.Restrict != nil || a.Mana.RestrictAb != nil {
		return false
	}
	c := *a
	m := *a.Mana
	m.Special, m.SpecialF = "", nil
	c.Mana = &m
	return plainTapMana(&c, true)
}

// primaryMana reports whether a is a first mana ability the kernel's pain
// source models as the normal alternative (pcard.manaCol): {C}, one WUBRG
// colour, or a two-colour {T} choice (Produced$ Combo W B).
func primaryMana(a *lAbility) bool {
	c := a.Mana.Colors
	if a.Mana.Each {
		return false
	}
	if a.Mana.Choice {
		return len(c) == 2 && strings.Trim(c, "WUBRG") == ""
	}
	return c == "C" || len(c) == 1 && strings.Contains("WUBRG", c)
}

// anyRFiltMana: three mana abilities, a plain {T} one of one fixed colour,
// a restricted {T}: Add one mana of any colour, then a {N}, {T}: Add one mana
// of any colour (White Lotus Hideout), in that printed order: the order of
// gorge's "choose a mana ability" ask the kernel's search mirrors. The
// kernel plans the source as one slot, a restricted planner source
// (mana_restrictedsource.go) and a filter gap, its search branching every
// answer of that ask (mana_mixsrc.go) as gorge's prefix search does; the
// per-ability loop still enforces that each is a modelled gap.
func anyRFiltMana(ms []*lAbility) bool {
	if len(ms) != 3 || len(ms[0].Mana.Colors) != 1 || ms[0].Mana.Choice {
		return false
	}
	r, f := ms[1], ms[2]
	return plainTapMana(r, false) && r.Mana.Restrict != nil && r.Mana.Colors == "WUBRG" && !r.Mana.Choice &&
		r.Mana.Amount.Kind == vConst && r.Mana.Amount.N == 1 &&
		plainTapMana(f, true) && f.Mana.Restrict == nil && f.Mana.Colors == "WUBRG"
}

// redundantMana reports whether a, a mana ability after the card's first,
// never offers what first does not, in the kernel's two-player games
// (twoSeat-at-caller: the opponents-attacked arm marks the card): a
// plain {T} ability of any one colour, unrestricted, whose amount is
//
//   - a read over commanders (Command Chair's greatest mana value among
//     commanders you control): no permanent is a commander, so it adds
//     nothing and gorge's planner lists no alternative for it;
//   - the opponents its controller attacked this turn (The Beacon of
//     Kahless), at most one, beside a first ability adding one mana of any
//     colour: nothing, or what the first adds.
//
// gorge's payment window asks which ability to activate and the oracle
// answers the first (refuseMultiMana), so the card plays as its first
// ability alone.
func redundantMana(first, a *lAbility) bool {
	if !plainTapMana(a, false) || a.Mana.Colors != "WUBRG" || a.Mana.Choice || a.Mana.Each ||
		a.Mana.Restrict != nil || a.Mana.RestrictAb != nil || a.Mana.NonSpell {
		return false
	}
	v := a.Mana.Amount
	if v.Add != 0 || v.Neg {
		return false
	}
	switch v.Kind {
	case vCount, vValidAgg:
		return commanderFilter(v.F)
	case vOppsAttacked:
		return v.Mul == 0 && v.Div == 0 && plainTapMana(first, false) && unrestrictedOne(first) &&
			first.Mana.Colors == "WUBRG" && !first.Mana.Choice && !first.Mana.Each
	}
	return false
}

// commanderFilter reports whether every clause of f requires a commander
// (IsCommander, kop.W7IsCommander): the second ability is then dropped on the
// assumption that nothing matches the filter. The cmd5 pool guard reads the
// counter.
func commanderFilter(f *lFilter) bool {
	if f == nil || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat != cmpWord7 || c.CmpN != int(kop.W7IsCommander) {
			return false
		}
	}
	noteCmdShortcut(cmdShortcutCommanderMana)
	return true
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
func manaGapModelled(first, a *lAbility) bool {
	switch {
	case dynamicLargerSameColour(first, a):
		return true
	case tapXGapMana(a):
		// {T}, tap an untapped creature you control: one mana of any
		// colour, unrestricted, beside a plain {T} first ability (Scene of
		// the Crime): gorge's census leaves it a relaxable once-only gap
		// its prefix search activates (the kernel's tap-another gap kind,
		// read at the card's tapX ability, mana_tapanother.go tapXAb)
		return a.Mana.Colors == "WUBRG" && !a.Mana.Choice && unrestrictedOne(a) && len(first.Mana.Colors) == 1 && !first.Mana.Choice
	case ctrTapGapMana(a):
		// {T}, remove N counters from this: a fixed colour of ManaN mana,
		// unrestricted, beside a plain {T} first ability of one colour or
		// any (Weather Maker's {C}{C}): gorge's census leaves it a relaxable
		// once-only gap its prefix search activates (the kernel's counter
		// gap kind, read at the card's ctrAb ability, mana_countercost.go)
		return ctrTapGapModelled(first, a)
	case restrictedFirstPair(first, a):
		// a spell-only first ability beside an activation-only larger one of
		// the same colour (Steelswarm Operator): the kernel plans each
		// restricted ability for the payment class its half admits
		return true
	case activationOnlyTapMana(a):
		// a chosen or fixed larger activation-only ability beside a same-colour
		// first one (Sunken Citadel): the kernel's restricted pair unit
		return restrictedFixedPair(first, a)
	case plainTapMana(a, false):
		// a restricted {T} ability, amount 1 (Courtyard, whose restriction
		// also names activated abilities; the Villages)
		// of one colour or any colour (the kernel's anyR branches on all
		// five: Tournament Grounds' {R}, {W} or {B} is not modelled)
		if a.Mana.Restrict == nil || a.Mana.Amount.Kind != vConst {
			return false
		}
		if a.Mana.Amount.N == 1 {
			return len(a.Mana.Colors) == 1 || a.Mana.Colors == "WUBRG"
		}
		// two or more mana of any one colour (Produced$ Any with a literal
		// Amount$): gorge's planner prices it as one alternative per colour
		// beside the first ability's, each of the larger amount, which the
		// kernel's restricted pair unit (mana_restrictedsource.go) plans.
		// Its colours must hold the first ability's, so the larger
		// alternative serves everything the smaller does. A Combo of any
		// colours is no planner alternative: a gap (restrictedComboModelled).
		if a.Mana.Choice {
			return restrictedComboModelled(first, a)
		}
		if restrictedFixedPair(first, a) {
			return true
		}
		return a.Mana.Colors == "WUBRG" && !a.Mana.Choice && a.Mana.Amount.N <= maxRestrictedAmount &&
			!first.Mana.Choice && (first.Mana.Colors == "WUBRG" || len(first.Mana.Colors) == 1 && strings.Contains("WUBRG", first.Mana.Colors))
	case plainTapMana(a, true):
		// {N}, {T}: one mana of any colour, unrestricted (Hidden Grotto;
		// Three Tree City's dynamic amount), or a literal amount above one
		// in any combination (Baxter Building's {4},{T}: four mana, the
		// kernel's combination filt)
		amt := a.Mana.Amount
		if a.Mana.Restrict != nil || a.Mana.RestrictAb != nil || a.Mana.Colors != "WUBRG" {
			return false
		}
		if amt.Kind == vConst && amt.N == 1 || amt.Kind > vConst {
			return true
		}
		return a.Mana.Choice && amt.Kind == vConst && amt.N > 1 && amt.N <= maxComboMana
	}
	return false
}

// tapXGapMana: a is a {T} ability whose cost also taps exactly one untapped
// permanent the controller controls, its cost otherwise plain.
func tapXGapMana(a *lAbility) bool {
	if a.Cost.TapN != 1 || a.Cost.TapPowN != 0 {
		return false
	}
	b := *a
	b.Cost.TapN, b.Cost.TapF = 0, nil
	return plainTapMana(&b, false)
}

// ctrTapGapMana: a is a {T} ability whose cost also removes counters from its
// source, its cost otherwise plain.
func ctrTapGapMana(a *lAbility) bool {
	if a.Cost.SubCtr < 1 || a.TurnOnce {
		return false
	}
	b := *a
	b.Cost.SubCtr, b.Cost.SubCtrKnd = 0, ""
	return plainTapMana(&b, false)
}

// ctrTapGapModelled: the counter-cost {T} ability a beside the plain first
// ability first is the shape kop.SecondCtrTapManaAb reads: a fixed colour of a
// literal amount, unrestricted, the first ability one mana of one colour or
// any (unrestricted, not a choice).
func ctrTapGapModelled(first, a *lAbility) bool {
	m, f := a.Mana, first.Mana
	return len(m.Colors) == 1 && !m.Choice && !m.Each && m.Restrict == nil && m.RestrictAb == nil && m.Amount.Kind == vConst && m.Amount.N >= 1 && m.Amount.N <= maxRestrictedAmount &&
		!f.Choice && (len(f.Colors) == 1 || f.Colors == "WUBRG")
}

// manaGapWhy names what a, a second mana ability manaGapModelled refuses,
// needs beyond the kernel's one-unit-per-permanent source model (the
// reasons the kernel's lockstep games and activation walk show):
//
//   - a cost beyond {T} other than the two below (a sacrifice, a mana
//     payment with a rider, ...): the kernel's mana walk cannot activate it
//     (a first and only all-at-once "{T}, Sacrifice another <filter>" is
//     modelled: refuseEachMana's sacEachMana);
//   - a cost of {T} plus removing counters other than ctrTapGapModelled's
//     (Vivid Crag's any colour, Throne of Makindi's restricted two): gorge's
//     census leaves it a relaxable once-only gap whose prefix script
//     activates the source, answers the ability ask with the second ability,
//     then its colour; the kernel's counter gap kind (mana_countercost.go)
//     reads a second ability only for a fixed unrestricted colour.
//     (Weather Maker's {C}{C} is modelled: ctrTapGapMana; so is the
//     tap-another shape, Scene of the Crime: tapXGapMana.)
//   - a spend-restricted ability of several mana of a fixed colour other than
//     the first ability's, or above maxRestrictedAmount (the same colour up to
//     that is Tablet of Discovery's modelled pair, restrictedFixedPair): the
//     kernel's restricted pair unit plans only an any-colour larger
//     alternative beyond it, so the planner taps differently from gorge's;
//   - an evaluated amount of the first ability's fixed colour the kernel's
//     per-count pair view does not play (dynamicLargerSameColour models
//     Itlimoc, Cradle of the Sun): the kernel plans the first ability alone,
//     gorge's planner the larger of the two.
func manaGapWhy(first, a *lAbility) string {
	m := a.Mana
	if activationOnlyTapMana(a) && !restrictedFirstPair(first, a) && !restrictedFixedPair(first, a) {
		return "an activation-only restricted ability of several mana: the kernel plans it only as a chosen or same-colour pair of two to four mana (or an activation-only larger ability beside a spell-only first one of that colour)"
	}
	switch {
	case !plainTapMana(a, false) && !plainTapMana(a, true):
		if a.Cost.SubCtr != 0 {
			return "a cost beyond {T} the kernel's mana walk cannot activate: its counter gap kind reads a second ability only for a fixed unrestricted colour"
		}
		return "a cost beyond {T} the kernel's mana walk cannot activate"
	case m.Restrict != nil && m.Amount.Kind == vConst && m.Amount.N > 1 && len(m.Colors) == 1 && m.Colors != "WUBRG" && !m.Choice:
		return "a restricted fixed colour of several mana: the kernel's restricted pair unit plans only an any-colour one or the first ability's own colour up to four"
	case m.Amount.Kind > vConst && len(m.Colors) == 1 && m.Colors == first.Mana.Colors && plainTapMana(a, false):
		return "an evaluated amount of the first ability's colour: the kernel plans the first ability alone"
	}
	return "a shape beyond the restricted, filter and pain gaps"
}

// activationOnlyTapMana: a is a plain {T} mana ability whose spend
// restriction names activated abilities only (RestrictValid$ Activated.<spec>;
// its spell half is nil before build_ab synthesizes the no-spell marker), its
// amount and production aside. Chosen reads as a production, not a cost.
func activationOnlyTapMana(a *lAbility) bool {
	if a == nil || a.Mana == nil || a.Mana.RestrictAb == nil || a.Mana.Restrict != nil || a.Mana.NonSpell {
		return false
	}
	c := *a
	m := *a.Mana
	m.RestrictAb = nil
	if m.Colors == "Chosen" {
		m.Colors = "C"
	}
	c.Mana = &m
	return plainTapMana(&c, false)
}

// spellOnlyMana: a's spend restriction names spells only.
func spellOnlyMana(a *lAbility) bool {
	return a != nil && a.Mana != nil && a.Mana.Restrict != nil && a.Mana.RestrictAb == nil && !a.Mana.NonSpell
}

// restrictedFixedPair: a, a restricted {T} ability of several literal mana
// (at most maxRestrictedAmount) of one fixed colour or the first ability's
// chosen colour, beside a first ability of that same colour, is the kernel's
// restricted pair unit with a single-colour larger alternative
// (mana_restrictedsource.go prepRestrSrc, restrPairUnit). Either a spell-only
// restriction (Tablet of Discovery's {R}{R} beside {R}) or an activation-only
// one (Sunken Citadel's chosen {C}{C} beside its chosen {C}); a different
// colour, a both-halves restriction, or a larger amount is not modelled.
func restrictedFixedPair(first, a *lAbility) bool {
	if a.Mana.Choice || first.Mana.Choice || a.Mana.Each || first.Mana.Each || a.Mana.NonSpell ||
		a.Mana.Amount.Kind != vConst || a.Mana.Amount.N < 2 || a.Mana.Amount.N > maxRestrictedAmount {
		return false
	}
	if !(spellOnlyMana(a) || activationOnlyTapMana(a)) {
		return false
	}
	if chosenTapMana(first) && a.Mana.Colors == "Chosen" && a.Mana.Special == "" {
		return true
	}
	return plainTapMana(first, false) && unrestrictedOne(first) && len(a.Mana.Colors) == 1 &&
		strings.Contains("WUBRG", a.Mana.Colors) && a.Mana.Colors == first.Mana.Colors
}

// restrictedFirstPair: first is a spell-only restricted {T} ability of one
// fixed colour, then a is an activation-only restricted {T} ability of the
// same colour and two to maxRestrictedAmount mana (Steelswarm Operator's
// artifact-spell {U} then artifact-activation {U}{U}). The kernel records both
// abilities and plans whichever half its payment class admits
// (mana_restrictedsource.go prepRestrSrc, restrViewW).
func restrictedFirstPair(first, a *lAbility) bool {
	if first.Mana.Choice || a.Mana.Choice || first.Mana.Each || a.Mana.Each || first.Mana.NonSpell || a.Mana.NonSpell ||
		!spellOnlyMana(first) || !plainTapMana(first, false) || !activationOnlyTapMana(a) ||
		first.Mana.Amount.Kind != vConst || first.Mana.Amount.N != 1 ||
		a.Mana.Amount.Kind != vConst || a.Mana.Amount.N < 2 || a.Mana.Amount.N > maxRestrictedAmount {
		return false
	}
	return len(first.Mana.Colors) == 1 && strings.Contains("WUBRG", first.Mana.Colors) && a.Mana.Colors == first.Mana.Colors
}

// restrictedComboModelled: a, a restricted {T} ability of a Combo of any
// colours, is the one shape the kernel's wide gap step models beside a first
// ability (Hermitic Herbalist): exactly maxRestrictedCombo mana, a spell-only
// restriction, and a first ability that is a plain unrestricted {T} one mana
// of any colour. gorge's prefix search activates the source, asks which
// ability, then the first's colour or the combination's per-unit allocation
// (kernel mana_restrictedcombo.go). Any other Combo stays refused.
func restrictedComboModelled(first, a *lAbility) bool {
	return a.Mana.Colors == "WUBRG" && a.Mana.Amount.N == maxRestrictedCombo && a.Mana.RestrictAb == nil && !a.Mana.NonSpell &&
		!a.Mana.Each && first.Mana.Colors == "WUBRG" && !first.Mana.Choice && !first.Mana.Each
}

// maxRestrictedCombo is the only literal amount of a restricted Combo of any
// colours the kernel models beside a first ability: its answer walk is the
// fifteen two-colour multisets (kernel wideAnswer).
const maxRestrictedCombo = 2

// maxRestrictedAmount bounds the literal amount of a restricted any-colour
// ability the kernel's pair unit plans (its planner's unit counts are bytes
// of small width, and a larger Produced$ Any amount has no card).
const maxRestrictedAmount = 4

// maxComboMana bounds the literal amount of a combination filter (Baxter
// Building's four mana in any combination). gorge's scriptAnswers enumerates
// at most 64 per-unit combinations, so the kernel's search does too.
const maxComboMana = 4

// dynamicLargerSameColour reports the pair refuseMultiMana names: a plain
// unrestricted {T}: Add one mana of a fixed colour, then a plain {T}: Add that
// same colour for each permanent matching a filter that tapping cannot change
// (Itlimoc, Cradle of the Sun: a creature you control). gorge prices the
// evaluated ability once, against a probe with every permanent tapped
// (paymentPlanStableAmount), so the amount is the same at planning and at
// activation only when the filter reads no tapped state.
func dynamicLargerSameColour(first, a *lAbility) bool {
	m, f := a.Mana, a.Mana.Amount
	if !plainTapMana(first, false) || !unrestrictedOne(first) || !plainTapMana(a, false) || a.Mana.Restrict != nil || a.Mana.RestrictAb != nil {
		return false
	}
	if len(first.Mana.Colors) != 1 || !strings.Contains("WUBRG", first.Mana.Colors) || first.Mana.Choice || first.Mana.Each {
		return false
	}
	if m.Colors != first.Mana.Colors || m.Choice || m.Each || f.Kind != vCount || f.F == nil {
		return false
	}
	return f.Add == 0 && f.Mul == 0 && f.Div == 0 && !f.Neg && len(f.Sub) == 0 && untappedStableFilter(f.F)
}

// untappedStableFilter: f reads no tapped or untapped clause, so tapping the
// permanents of a payment cannot change the count it matches.
func untappedStableFilter(f *lFilter) bool {
	for _, c := range f.Clauses {
		if c.Flags&(fTapped|fUntapped) != 0 {
			return false
		}
	}
	return len(f.Clauses) > 0
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
	if len(ms) != 1 {
		for _, a := range ms {
			if metalcraftGate(a.ActCond) {
				// only a card's lone mana ability is verified with this gate
				lc.refuseNode("card", "metalcraft-gated mana ability beside another mana ability")
				return true
			}
		}
	}
	if len(ms) == 2 && fixedColourFiltMana(ms[0], ms[1]) {
		// a plain {T}: Add {c} and a Station-gated {c}, {T}: Add {c} for each
		// ... (Evendo, Uthros): gorge's census lists the first ability alone
		// while the grant is off and the second as a paid gap while it is on
		// (the kernel's gated fixed-colour filter, filterTapMana)
		return true
	}
	// the gate may be the only thing off the plain shape; a card's only mana
	// ability may also make a literal amount above one of one fixed colour
	// (Temple of the False God's {C}{C}): its gate-on view is the plain
	// source's, manaN of it (prepManaGate)
	plain := func(a *lAbility) bool {
		c := *a
		c.ActCond = lCond{}
		if m := *c.Mana; len(ms) == 1 && len(m.Colors) == 1 && !m.Choice && !m.Each && m.Amount.Kind == vConst && m.Amount.N > 1 && m.Amount.N <= maxEachMana {
			m.Amount.N = 1
			c.Mana = &m
		}
		return plainTapMana(&c, false) && unrestrictedOne(&c)
	}
	if len(ms) == 2 && ms[0].ActCond.Kind == cNone && plain(ms[0]) && ms[0].Mana.Colors == "C" && ms[1].ActCond.Kind == cPresent {
		c := *ms[1]
		c.ActCond = lCond{}
		if lifeTapMana(&c) {
			// {T}: Add {C}, then a gated {T}, Pay N life: Add one mana of
			// any colour (Spire of Industry): gorge's census lists the
			// {C} alternative alone while the gate is off, and Starting
			// Town's last-resort life alternative beside it while it holds;
			// the kernel's gate-off view drops the painful alternative
			// (prepManaGate)
			return true
		}
	}
	if gatedLargerSameColour(ms) {
		// Whisperer of the Wilds: gorge's planner lists the source with
		// both alternatives while the gate holds and takes the one that
		// ranks best per payment (fewest sources, then least surplus); the
		// payment window answers the first ability. The kernel's gate-on
		// view is the restricted source's pair unit without the restricted
		// float (prepManaGate: the first ability's manaN beside the larger
		// one's wideN), the affordability bounds read the larger amount.
		// An amount above maxEachMana is outside the byte-sized unit.
		if ms[1].Mana.Amount.N > maxEachMana {
			lc.refuseNode("card", "gated {T} ability making more than four mana of the colour a plain {T} ability makes one of")
		}
		return true
	}
	for _, a := range ms {
		if !plain(a) {
			lc.refuseNode("card", "gated mana ability beside a mana ability or cost not modelled (plain {T}, amount 1, unrestricted only)")
			return true
		}
		if a.ActCond.Kind != cNone && a.ActCond.Kind != cPresent && !metalcraftGate(a.ActCond) {
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

// metalcraftGate: c is Activation$ Metalcraft as lowered (bareCondition): the
// controller's artifact count at least three. The kernel reads a gated mana
// ability's Trig.Cond with condEval per call (prepManaGate, manaGateView), so
// a count compare gates the source's view the way an IsPresent$ gate does.
func metalcraftGate(c lCond) bool {
	return c.Kind == cValue && c.A.Kind == vCount && c.A.F != nil && c.Op == opGE && c.B.isConst() && c.B.N == 3
}

// fixedColourFiltMana: first is a plain unrestricted {T}: Add one mana of one
// WUBRG colour and a is the card's own counters-gated "{c}, {T}: Add {c} for
// each ...": one coloured pip of the colour it makes, a dynamic amount, no
// restriction (the Station-granted Planets, stationGrantMana). The gate is
// the Station threshold, the only one the filter's gap view reads.
func fixedColourFiltMana(first, a *lAbility) bool {
	if first.ActCond.Kind != cNone || !plainTapMana(first, false) || !unrestrictedOne(first) || first.Mana.Choice ||
		first.Mana.Each || len(first.Mana.Colors) != 1 || !strings.Contains("WUBRG", first.Mana.Colors) {
		return false
	}
	if a.ActCond.Kind != cPresent {
		return false
	}
	if _, ok := stationThreshold(a.ActCond.F); !ok || a.ActCond.F.Clauses[0].CmpCounter != "CHARGE" || a.ActCond.F.Clauses[0].CmpOp != opGE ||
		a.ActCond.Op != opGE || !a.ActCond.A.isConst() || a.ActCond.A.N != 1 {
		return false
	}
	c := *a
	c.ActCond = lCond{}
	m := c.Cost.Mana
	pip, n := -1, 0
	for i, v := range m.Pips {
		if v != 0 {
			pip, n = i, n+v
		}
	}
	if n != 1 || m.Pips[pip] != 1 || m.Generic != 0 || m.X || m.XX || m.C != 0 || len(m.Hybrid) != 0 {
		return false
	}
	c.Cost.Mana.Pips = [5]int{}
	if !plainTapMana(&c, false) || c.Mana.Choice || c.Mana.Each || c.Mana.Restrict != nil || c.Mana.RestrictAb != nil || c.Mana.NonSpell ||
		c.Mana.Colors != string("WUBRG"[pip]) || c.Mana.Amount.Kind <= vConst {
		return false
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
// program) of one colour, a choice of two or three, or any colour.
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
	case n == 2, n == 3:
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

// refuseEachMana guards a mana ability that adds several colours at once
// (lMana.Each: Timeless Lotus's {W}{U}{B}{R}{G}). gorge's planner prices
// such a source as one fixed-production alternative whose Alt.Mana holds
// every colour (pay appendUnitAlternatives); the kernel models it as a unit
// of its first colour with the rest a fixed vector beside it, the shape of a
// tap's extra mana (kernel mana_tapextra.go, pcard.manaXv). Only the plain
// shape is modelled: the card's one mana ability, a bare {T}, unrestricted,
// a literal amount of each colour. It reports whether it refused.
func (lc *lowerCtx) refuseEachMana(ms []*lAbility) bool {
	for _, a := range lc.out.Abilities {
		if a.Kind != akMana || a.Mana == nil || !a.Mana.Each {
			continue
		}
		m := a.Mana
		switch {
		case a.Latent:
			lc.refuseNode("card", "granted mana ability adding several colours at once")
		case len(ms) != 1:
			lc.refuseNode("card", "mana ability adding several colours at once beside another mana ability")
		case m.RestrictAb != nil || m.NonSpell:
			// an ability-only or non-spell restriction on an all-at-once
			// ability has no measured repro. A spell restriction (Troyan,
			// Gutsy Explorer; Tin Street Gossip's Static.isTurnFaceUp clause)
			// is a restricted planner unit (kernel mana_restrictedsource.go).
			lc.refuseNode("card", "mana ability adding several colours at once under a spend restriction")
		case !plainTapMana(a, false) && !ctrEachMana(a) && !filtEachMana(a) && !sacEachMana(a):
			// gorge's planner defers any cost beyond {T} (a filter's fee, a
			// sacrifice of another permanent, removed counters): a census gap.
			// The kernel's gap searches model the counter removal (ctrEachMana),
			// a generic fee (filtEachMana) and {T} plus a sacrifice of exactly
			// one other permanent (sacEachMana, Bolg's Company); a sacrifice of
			// another permanent in any other shape (a fee beside it, two of them)
			// has no gap kind (manaGapWhy).
			if a.Cost.Sac != nil && !a.Cost.SacSelf {
				lc.refuseNode("card", "mana ability adding several colours at once from a cost beyond {T}: a sacrifice of another permanent (no sacrifice gap kind in the kernel's prefix search)")
			} else {
				lc.refuseNode("card", "mana ability adding several colours at once from a cost beyond {T}")
			}
		case !m.Amount.isConst() || m.Amount.N < 1 || m.Amount.N > maxEachMana:
			lc.refuseNode("card", "mana ability adding several colours at once, %v of each", m.Amount)
		default:
			continue
		}
		return true
	}
	return false
}

// ctrEachMana: a is "Remove N counters from this: Add ... of each colour.
// Activate only once each turn." (Ramos, Dragon Engine), the one cost beyond
// {T} the kernel models all at once: its relaxed census alternative is one
// free fixed production (gorge paymentPlanRelaxedAlternatives), activated by
// the counter-cost gap (kernel mana_countercost.go, kop.CtrEachManaAb).
func ctrEachMana(a *lAbility) bool {
	if a.Cost.SubCtr < 1 || a.Cost.Tap || !a.TurnOnce {
		return false
	}
	c := *a
	c.Cost.Tap, c.Cost.SubCtr, c.Cost.SubCtrKnd, c.TurnOnce = true, 0, "", false
	return plainTapMana(&c, false)
}

// sacEachMana: a is "{T}, Sacrifice another <filter>: Add {c1}{c2}..."
// (Bolg's Company's "{T}, Sacrifice another Goblin: Add {B}{R}"): {T} plus a
// sacrifice of exactly one other permanent, no mana, otherwise a plain {T}
// all-at-once production. gorge's census defers it and relaxes it to one free
// fixed production (paymentPlanRelaxedAlternatives); the kernel's tap-another
// search activates it (kop.SacXEachManaAb, mana_tapanother.go).
func sacEachMana(a *lAbility) bool {
	if !a.Cost.Tap || a.Cost.Sac == nil || a.Cost.SacSelf || a.Cost.SacN != 1 || a.Cost.SacX || a.Cost.SacExile {
		return false
	}
	c := *a
	c.Cost.Sac, c.Cost.SacN = nil, 0
	m := a.Mana
	if !plainTapMana(&c, false) || m.Choice || m.Restrict != nil || m.RestrictAb != nil || m.NonSpell ||
		!m.Amount.isConst() || m.Amount.N != 1 || len(m.Colors) < 2 || strings.Trim(m.Colors, "WUBRG") != "" {
		return false
	}
	for i := 0; i < len(m.Colors); i++ {
		if strings.IndexByte(m.Colors[i+1:], m.Colors[i]) >= 0 {
			return false // a duplicated colour (one colour twice): gorge's relaxed production differs
		}
	}
	return true
}

// filtEachMana: a is "{N}, {T}: Add {c1}{c2}..." (Skycloud Expanse's {1},{T}:
// Add {W}{U}): a generic-only fee, one of each of two or more distinct WUBRG
// colours, unrestricted, the card's first mana ability. gorge's planner census
// skips its paid cost; its relaxed alternative is one fixed production (fee
// charged as generic, once: its {T}), which the kernel's first-only filter gap
// activates (kop.FilterEachManaAb, mana_filtereach.go).
func filtEachMana(a *lAbility) bool {
	m := a.Mana
	if !plainTapMana(a, true) || a.Cost.Mana.Generic < 1 || m.Choice || m.Restrict != nil || m.RestrictAb != nil || m.NonSpell ||
		!m.Amount.isConst() || m.Amount.N != 1 || len(m.Colors) < 2 || strings.Trim(m.Colors, "WUBRG") != "" {
		return false
	}
	for i := 0; i < len(m.Colors); i++ {
		if strings.IndexByte(m.Colors[i+1:], m.Colors[i]) >= 0 {
			return false
		}
	}
	return true
}

// filterHasSpecial reports whether f carries a RestrictValid$ Static.*
// alternative (a clause with Special set).
func filterHasSpecial(f *lFilter) bool {
	if f == nil {
		return false
	}
	for i := range f.Clauses {
		if f.Clauses[i].Special != 0 {
			return true
		}
	}
	return false
}

// maxEachMana bounds the amount of each colour an all-at-once mana ability
// adds (the kernel's unit and extra vector are bytes; no card makes more).
const maxEachMana = 4

// gatedLargerSameColour reports the pair refuseGatedMana names: a plain
// unrestricted {T}: Add one mana of a fixed colour, then a gated {T}: Add a
// literal two or more of that same colour.
func gatedLargerSameColour(ms []*lAbility) bool {
	if len(ms) != 2 || ms[0].ActCond.Kind != cNone || ms[1].ActCond.Kind != cPresent {
		return false
	}
	c := *ms[1]
	c.ActCond = lCond{}
	m := c.Mana
	return plainTapMana(ms[0], false) && unrestrictedOne(ms[0]) && plainTapMana(&c, false) &&
		len(ms[0].Mana.Colors) == 1 && !ms[0].Mana.Choice && !ms[0].Mana.Each &&
		m.Colors == ms[0].Mana.Colors && !m.Choice && !m.Each && m.Amount.Kind == vConst && m.Amount.N > 1
}
