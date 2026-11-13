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
// reflectsStableObjects); the census resolves the colours with the
// resolution's own walk (effects.ManaReflectedObjects) and drops {C}
// (PaymentPlanReflectedCounts). The kernel reads exactly: Cost$ T,
// ColorOrType$ Color, ReflectProperty$ Is with Valid$ <filter>[,<filter>]
// each with YouCtrl, or ReflectProperty$ Produce with each with OppCtrl.
// Anything else is refused.
func (lc *lowerCtx) lowerManaReflected(sa *cards.SA, ps *params, ab *lAbility) error {
	if ps.str("ColorOrType") != "Color" {
		return refuse("ManaReflected ColorOrType$ %q", ps.str("ColorOrType"))
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

// lowerManaReflectedProduce lowers the opponents'-production shape (Valid$
// <filter>[,<filter>] each with OppCtrl): one mana of any colour among the
// matched permanents' cards' producible colours (CardDef.ProduceCol; kernel
// mana_reflected.go produceColours, gorge effects.producibleSymbols). A
// mixed YouCtrl list would need two walks, the kernel's one walk is the
// opponents' alone, so every alternative must carry OppCtrl; gorge's planner
// admits the mixed list (reflectsStableObjects), no corpus card carries one.
func (lc *lowerCtx) lowerManaReflectedProduce(sa *cards.SA, ps *params, ab *lAbility) error {
	valid := ps.str("Valid")
	if valid == "" || strings.HasPrefix(valid, "Defined.") {
		return refuse("ManaReflected Valid$ %q", valid)
	}
	for _, alt := range strings.Split(valid, ",") {
		if _, props, ok := strings.Cut(strings.TrimSpace(alt), "."); !ok ||
			!slices.Contains(strings.Split(props, "+"), "OppCtrl") {
			// gorge's shape needs the opponent read on every alternative
			// (reflectsStableObjects); the kernel's one walk is the
			// opponents' matching permanents (produceColours)
			return refuse("ManaReflected Valid$ %q without OppCtrl", valid)
		}
	}
	f, err := lc.filter(valid)
	if err != nil {
		return err
	}
	m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "Reflected", Produce: true, SpecialF: f}
	return lc.finishMana(sa, ps, ab, m)
}

// reflectedProducedReplacer strips a Produced$ list's braces and spaces
// (gorge effects manaReflectedReplacer).
var reflectedProducedReplacer = strings.NewReplacer("{", "", "}", "", " ", "")

// producibleMask is the colours the face's printed mana abilities could
// produce ({C} dropped — the only consumer, a Produce-mode reflected
// ability's ColorOrType$ Color, never admits it), gorge
// effects.producibleSymbols over Face.ManaAbilities exactly: a blank,
// "Any" or "Combo Any" Produced$ is every colour, "Combo <colours>" exactly
// those, a plain brace/space-stripped list its coloured runes. Stored on
// CardDef.ProduceCol; the kernel unions it over the matched permanents at
// each use.
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
		for _, r := range reflectedProducedReplacer.Replace(p) {
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
		// over the filter's colours (dirac mana_imprint.go prepManaImprint)
		if !reflTapMana(ms[0]) {
			lc.refuseNode("card", "lone reflected mana ability with a cost beyond {T}")
		}
		return true
	}
	switch {
	case len(ms) != 3 || at != 2:
		lc.refuseNode("card", "reflected mana ability outside {C}, restricted any colour, reflected")
	case !plainTapMana(ms[0], false) || !unrestrictedOne(ms[0]) || ms[0].Mana.Colors != "C" || ms[0].Mana.Choice:
		lc.refuseNode("card", "reflected mana ability beside a first ability other than {T}: Add {C}")
	case !plainTapMana(ms[1], false) || ms[1].Mana.Colors != "WUBRG" || ms[1].Mana.Restrict == nil || ms[1].Mana.RestrictAb != nil ||
		ms[1].Mana.Amount.Kind != vConst || ms[1].Mana.Amount.N != 1:
		lc.refuseNode("card", "reflected mana ability beside a second ability other than a restricted any colour")
	case !reflTapMana(ms[2]):
		lc.refuseNode("card", "reflected mana ability with a cost beyond {T}")
	}
	return true
}
