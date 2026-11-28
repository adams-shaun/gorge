package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// AB$ ManaReflected in its two planned colour-read shapes over permanents:
// the controller's own ("add one mana of any color among legendary
// permanents you control": Plaza of Heroes, ReflectProperty$ Is, every
// Valid$ alternative YouCtrl) and the opponents' production ("add one mana
// of any color that a land an opponent controls could produce": Exotic
// Orchard, Fellwar Stone, Quirion Explorer, Sylvok Explorer,
// ReflectProperty$ Produce, every Valid$ alternative OppCtrl). gorge
// pay.PaymentPlanReflectedShape admits both over a Valid$ whose every
// alternative carries the matching control property (YouCtrl or OppCtrl,
// reflectsStableObjects); the census resolves the symbols with the
// resolution's own walk (effects.ManaReflectedObjects; PaymentPlanReflectedCounts
// keeps a ColorOrType$ Type read's {C}). The kernel reads exactly: Cost$ T,
// ColorOrType$ Color, ReflectProperty$ Is with Valid$ <filter>[,<filter>]
// each with YouCtrl, or ReflectProperty$ Produce with each with OppCtrl or
// YouCtrl, its ColorOrType$ Type widening the read to {C}. Anything else is
// refused.
func (lc *lowerCtx) lowerManaReflected(sa *cards.SA, ps *params, ab *lAbility) error {
	if ct := ps.str("ColorOrType"); ct != "Color" && !(ct == "Type" && ps.str("ReflectProperty") == "Produce") {
		return refuse("ManaReflected ColorOrType$ %q", ct)
	}
	if ps.str("ReflectProperty") == "Produce" {
		return lc.lowerManaReflectedProduce(sa, ps, ab)
	}
	if ps.str("ReflectProperty") != "Is" {
		return refuse("ManaReflected ReflectProperty$ %q", ps.str("ReflectProperty"))
	}
	valid := ps.str("Valid")
	if valid == "Defined.Self" && lc.granted > 0 {
		// (Katilda, Dawnhart Prime) a GRANTED "{T}: Add one mana of any
		// of this creature's colors" reads the grantee's own colours
		// (pay/planreflected.go reflectsStableObjects); the
		// kernel's granted source takes the colours of the slot it is
		// granted to (kernel mana_owncolours.go).
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "ReflectedSelf"}
		return lc.finishMana(sa, ps, ab, m)
	}
	if valid == "Defined.Imprinted" && lc.imprintMox && lc.granted == 0 {
		// (Chrome Mox) the colours of the card the enters trigger exiled,
		// read through its reverse exiled-with record (lower_imprintmox.go;
		// kernel mana_imprint.go). The card records what it exiles and
		// its text names ExiledWithSource, so the record is the reverse one
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "ReflectedImprint"}
		lc.out.ExiledWith, lc.out.ExwWord = true, true
		return lc.finishMana(sa, ps, ab, m)
	}
	if rest, ok := strings.CutPrefix(valid, "Defined.ValidGraveyard "); ok {
		// (The Grey Havens) "Add one mana of any color among legendary
		// creature cards in your graveyard": gorge plans the graveyard scan
		// (pay.paymentPlanReflectedDefined's ValidGraveyard; planshape.go),
		// its walk every alive seat's graveyard in AliveFrom(0) order, the
		// spec matched with the controller as "you" (effects
		// reflectedDefinedExtras). The kernel's read is graveReflColours
		// (mana_reflected.go) over the two-ability reflected shape's views;
		// a lone one refuses (refuseReflectedMana: the imprint view has no
		// graveyard arm)
		f, err := lc.filter(strings.TrimSpace(rest))
		if err != nil {
			return err
		}
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "Reflected", Graveyard: true, SpecialF: f}
		return lc.finishMana(sa, ps, ab, m)
	}
	if valid == "" || strings.HasPrefix(valid, "Defined.") {
		return refuse("ManaReflected Valid$ %q", valid)
	}
	for _, alt := range strings.Split(valid, ",") {
		if !strings.Contains(alt, "+YouCtrl") {
			// gorge's shape needs the controller read on every alternative
			// (reflectsStableObjects); OppCtrl reads lands' production
			return refuse("ManaReflected Valid$ %q without YouCtrl", valid)
		}
	}
	f, err := lc.filter(valid)
	if err != nil {
		return err
	}
	m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "Reflected", SpecialF: f}
	return lc.finishMana(sa, ps, ab, m)
}

// lowerManaReflectedProduce lowers the production shape (Valid$
// <filter>[,<filter>] each with OppCtrl, Exotic Orchard's "a land an opponent
// controls", or YouCtrl, Gond Gate's "a Gate you control" and Star Compass's
// "a basic land you control"): one mana of any colour among the matched
// permanents' cards' producible colours (CardDef.ProduceCol; kernel
// mana_reflected.go produceColours, gorge effects.producibleSymbols), plus
// {C} when ColorOrType$ Type and a matched permanent makes it (kernel
// producesColourless; gorge effects.ManaReflectedFold). The kernel's walk
// covers every seat's permanents and the filter's control property keeps one
// side out, the same read as gorge's object walk (effects.ManaReflectedObjects
// over AliveFrom(0)).
func (lc *lowerCtx) lowerManaReflectedProduce(sa *cards.SA, ps *params, ab *lAbility) error {
	valid := ps.str("Valid")
	widenType := ps.str("ColorOrType") == "Type"
	if valid == "Defined.Sacrificed" {
		// (Squandered Resources) "Sacrifice a land: Add one mana of any type
		// that land could produce": the victim's producible symbols are read at
		// the activation (kernel mana_tapanother.go sacReflMask). The cost must
		// be exactly one sacrificed permanent and nothing else; the kernel's
		// sacrifice-another search asks the victim before the colour.
		c := ab.Cost
		if !widenType || c.Tap || c.Sac == nil || c.SacN != 1 || c.SacX || c.SacMore != nil ||
			c.Mana.Generic != 0 || c.Mana.Pips != [5]int{} || len(c.Mana.Hybrid) != 0 || c.Mana.X || c.Mana.C != 0 || c.Mana.Snow != 0 ||
			c.DiscardN != 0 || c.DiscardSelf || c.Loyalty != 0 || c.SubCtr != 0 || c.PayLife != 0 || c.TapN != 0 ||
			c.TapHost || c.SacHost || c.Forage != nil || c.ExileG != nil || c.Energy != 0 || c.ReturnSelf || c.ExileSelf || c.Return != nil {
			return refuse("ManaReflected Valid$ %q beside a cost beyond Sac<1/...>", valid)
		}
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "ReflectedSacrificed"}
		return lc.finishMana(sa, ps, ab, m)
	}
	if valid == "" || strings.HasPrefix(valid, "Defined.") {
		return refuse("ManaReflected Valid$ %q", valid)
	}
	for _, alt := range strings.Split(valid, ",") {
		if _, props, ok := strings.Cut(strings.TrimSpace(alt), "."); !ok ||
			!slices.Contains(strings.Split(props, "+"), "OppCtrl") && !slices.Contains(strings.Split(props, "+"), "YouCtrl") {
			// gorge's shape needs a control read on every alternative
			// (reflectsStableObjects); the kernel walks every seat's
			// permanents and the filter's own control property decides
			return refuse("ManaReflected Valid$ %q without OppCtrl or YouCtrl", valid)
		}
	}
	f, err := lc.filter(valid)
	if err != nil {
		return err
	}
	amt, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	if !amt.isConst() {
		// (Incubation Druid) "Add one mana of any type that a land you
		// control could produce. If it has a +1/+1 counter on it, add three
		// mana of that type instead": Amount$ Count$Compare Y GE1.3.1 over
		// the counters. gorge's planner defers the amount (pay
		// PaymentPlanReflectedShape rejects mr.Amount.Present): its census
		// leaves the source a gap, the fold prices the amount open, and the
		// prefix search evaluates it at the activation — the kernel's
		// AfManaDynamic + ManaV over the Combo Any plumbing (dirac
		// mana_reflamt.go)
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: amt, Special: "Reflected", Produce: true, WidenType: widenType, SpecialF: f}
		return lc.finishMana(sa, ps, ab, m)
	}
	if amt.N != 1 {
		return refuse("ManaReflected Amount$ %d", amt.N)
	}
	m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "Reflected", Produce: true, WidenType: widenType, SpecialF: f}
	return lc.finishMana(sa, ps, ab, m)
}

// reflectedProducedReplacer strips a Produced$ list's braces and spaces
// (gorge effects manaReflectedReplacer).
var reflectedProducedReplacer = strings.NewReplacer("{", "", "}", "", " ", "")

// producibleMask is the coloured mana the face's printed mana abilities could
// produce, gorge effects.producibleSymbols over Face.ManaAbilities exactly: a
// blank, "Any" or "Combo Any" Produced$ is every colour, "Combo <colours>"
// exactly those, a plain brace/space-stripped list its coloured runes when
// every rune is a known mana symbol (a non-symbol word fails closed, so
// "Combo ColorIdentity" contributes nothing). {C} is not in the mask: the
// kernel reads it from the matched cards' abilities when the reflected
// ability's ManaProduceType admits it (producesColourless), so a
// ColorOrType$ Type read keeps it and a Color read drops it. Stored on
// CardDef.ProduceCol; the kernel unions it over the matched permanents at each
// use.
func producibleMask(f *cards.Face) kop.ColorMask {
	if f == nil {
		return 0
	}
	var set kop.ColorMask
	for _, ma := range f.ManaAbilities() {
		p := effects.ManaOf(ma).Produced
		switch {
		case p == "" || p == "Any" || p == "Combo Any":
			set |= kop.ColAny
			continue
		}
		if cols, ok := effects.ComboColours(p); ok {
			for _, col := range cols {
				set |= colorMask(col)
			}
			continue
		}
		runes := reflectedProducedReplacer.Replace(p)
		valid := true
		for _, r := range runes {
			if !strings.ContainsRune(effects.ManaSymbols, r) {
				valid = false
				break
			}
		}
		if !valid {
			continue // a word, not a rune list: gorge's classifier fails closed
		}
		for _, r := range runes {
			if i := strings.IndexRune("WUBRG", r); i >= 0 {
				set |= 1 << uint(i)
			}
		}
	}
	return set
}

// reflTapMana: a is the lowered reflected ability, cost {T} alone.
func reflTapMana(a *lAbility) bool {
	if a.Mana == nil || a.Mana.Special != "Reflected" && a.Mana.Special != "ReflectedImprint" && a.Mana.Special != "ReflectedIdentity" {
		return false
	}
	return reflTapCost(&a.Cost) && len(a.Prog) == 0 && a.ActCond.Kind == cNone && a.Limit == 0 && !a.TurnOnce && a.Gate == nil && !a.Sorcery && !a.Loyalty
}

// reflTapLifeMana: a is the lowered reflected ability whose cost is exactly
// {T} plus PayLife<N> (Horizon of Progress). gorge's planner prices such an
// ability last-resort, Consequence{Life: N} (pay.PaymentPlanShapeTierOf's
// cost.Life arm), which the kernel's lone imprint view carries as pain
// (dirac mana_pain.go manaLifePain over prepManaImprint's view fields).
func reflTapLifeMana(a *lAbility) bool {
	if a.Mana == nil || a.Mana.Special != "Reflected" && a.Mana.Special != "ReflectedImprint" && a.Mana.Special != "ReflectedIdentity" {
		return false
	}
	return reflTapCostLife(&a.Cost) && len(a.Prog) == 0 && a.ActCond.Kind == cNone && a.Limit == 0 && !a.TurnOnce && a.Gate == nil && !a.Sorcery && !a.Loyalty
}

// reflTapCostLife is reflTapCost with exactly a PayLife<N> (N >= 1) admitted.
func reflTapCostLife(c *lCost) bool {
	if c.PayLife <= 0 {
		return false
	}
	m := &c.Mana
	if !c.Tap || c.SacSelf || c.Sac != nil || c.DiscardN != 0 || c.DiscardSelf || c.Loyalty != 0 || c.SubCtr != 0 ||
		c.TapN != 0 || c.TapHost || c.SacHost || c.Forage != nil || c.ExileG != nil || c.Energy != 0 || c.ReturnSelf || c.ExileSelf || c.Return != nil {
		return false
	}
	return m.Pips == [5]int{} && len(m.Hybrid) == 0 && !m.X && m.C == 0 && m.Generic == 0
}

// reflTapCost: c is {T} alone.
func reflTapCost(c *lCost) bool {
	m := &c.Mana
	if !c.Tap || c.SacSelf || c.Sac != nil || c.DiscardN != 0 || c.DiscardSelf || c.Loyalty != 0 || c.SubCtr != 0 || c.PayLife != 0 ||
		c.TapN != 0 || c.TapHost || c.SacHost || c.Forage != nil || c.ExileG != nil || c.Energy != 0 || c.ReturnSelf || c.ExileSelf || c.Return != nil {
		return false
	}
	return m.Pips == [5]int{} && len(m.Hybrid) == 0 && !m.X && m.C == 0 && m.Generic == 0
}

// refuseReflectedMana handles a card with a reflected mana ability. The
// kernel models exactly Plaza of Heroes' three: {T}: Add {C}; {T}: Add one
// mana of any colour, restricted (a Courtyard-like census gap, genPrep.anyR);
// {T}: the reflected colour. gorge's planner census lists one unit: the {C}
// alternative, the reflected colours (normal tier), and the restricted any
// (deferred); the kernel's view is the union {C} | the reflected colours
// (kernel mana_reflected.go). A reflected ability alone (Mox Amber) is the
// imprint mox's per-colour views over the filter's colours (kernel
// mana_imprint.go). It reports whether ms holds a
// reflected ability (the caller then has nothing to add).
func (lc *lowerCtx) refuseReflectedMana(ms []*lAbility) bool {
	for _, a := range ms {
		if a.Mana.Special != "ReflectedImprint" && a.Mana.Special != "ReflectedIdentity" {
			continue
		}
		// the imprint mox's one ability (lower_imprintmox.go): the kernel
		// models it alone, {T} its whole cost
		if len(ms) != 1 || !reflTapMana(a) {
			lc.refuseNode("card", "imprint-reflected mana ability beside another mana ability or with a cost beyond {T}")
		}
		return true
	}
	at := -1
	for i, a := range ms {
		if a.Mana.Special == "Reflected" {
			if at >= 0 {
				lc.refuseNode("card", "two reflected mana abilities")
				return true
			}
			at = i
		}
	}
	if at < 0 {
		return false
	}
	if len(ms) == 1 {
		// (Mox Amber) the filter-reflected ability alone, {T} its whole
		// cost: the kernel models it as the imprint mox's per-colour views
		// over the filter's colours (dirac mana_imprint.go prepManaImprint).
		// A {T} + PayLife<N> one (Horizon of Progress) lowers too: gorge's
		// planner prices it last-resort, Consequence{Life: N}
		// (pay.PaymentPlanShapeTierOf's cost.Life arm), which the kernel's
		// imprint view carries as pain (dirac mana_pain.go painUnit over
		// prepManaImprint's view fields)
		if !reflTapMana(ms[0]) && !reflTapLifeMana(ms[0]) {
			lc.refuseNode("card", "lone reflected mana ability with a cost beyond {T}")
		}
		if reflTapLifeMana(ms[0]) && ms[0].Mana.Amount.Kind != vConst {
			// a life cost with an Amount$ (no corpus card): the kernel's gap
			// discriminator (reflAmtManaAb) wants {T} alone, so the card would
			// take the one-mana imprint view while gorge defers the amount
			lc.refuseNode("card", "lone reflected Amount$ mana ability with a life cost")
		}
		if ms[0].Mana.Graveyard {
			// the imprint view's colours read the battlefield or the
			// imprint record only (mana_imprint.go reflColours's arms); a
			// graveyard scan lowers beside a plain first ability (The Grey
			// Havens) and alone it has no view
			lc.refuseNode("card", "lone reflected mana ability reading the graveyard")
		}
		return true
	}
	if len(ms) == 2 && at == 1 {
		// (Gond Gate) {T}: Add {C}; {T}: the reflected colour. gorge's census
		// lists the same one unit, {C} | the reflected colours, without the
		// restricted any colour's deferred gap
		switch {
		case !plainTapMana(ms[0], false) || !unrestrictedOne(ms[0]) || ms[0].Mana.Colors != "C" || ms[0].Mana.Choice:
			lc.refuseNode("card", "reflected mana ability beside a first ability other than {T}: Add {C}")
		case ms[1].Mana.Amount.Kind != vConst:
			// an Amount$ one (Incubation Druid's shape) beside another mana
			// ability: its census gap registers position-independently (gen.go
			// registersGapOther) but the window enabler is the first mana
			// ability's alone (gen.go j == firstManaAb, mana_reflamt.go), so
			// the gap would be registered yet never offered — gorge's manual
			// window lists every untapped source
			lc.refuseNode("card", "reflected Amount$ mana ability beside another mana ability")
		case !reflTapMana(ms[1]):
			lc.refuseNode("card", "reflected mana ability with a cost beyond {T}")
		}
		return true
	}
	switch {
	case len(ms) != 3 || at != 2:
		lc.refuseNode("card", "reflected mana ability outside {C}, reflected or {C}, restricted any colour, reflected")
	case !plainTapMana(ms[0], false) || !unrestrictedOne(ms[0]) || ms[0].Mana.Colors != "C" || ms[0].Mana.Choice:
		lc.refuseNode("card", "reflected mana ability beside a first ability other than {T}: Add {C}")
	case !plainTapMana(ms[1], false) || ms[1].Mana.Colors != "WUBRG" || ms[1].Mana.Restrict == nil || ms[1].Mana.RestrictAb != nil ||
		ms[1].Mana.Amount.Kind != vConst || ms[1].Mana.Amount.N != 1:
		lc.refuseNode("card", "reflected mana ability beside a second ability other than a restricted any colour")
	case ms[2].Mana.Amount.Kind != vConst:
		// an Amount$ one (Incubation Druid's shape) beside two mana abilities:
		// its census gap registers position-independently (gen.go
		// registersGapOther) but the window enabler is the first mana ability's
		// alone (gen.go j == firstManaAb, mana_reflamt.go), so the gap would be
		// registered yet never offered — gorge's manual window lists every
		// untapped source
		lc.refuseNode("card", "reflected Amount$ mana ability beside another mana ability")
	case !reflTapMana(ms[2]):
		lc.refuseNode("card", "reflected mana ability with a cost beyond {T}")
	}
	return true
}
