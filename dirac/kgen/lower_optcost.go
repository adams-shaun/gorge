package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// "As an additional cost to cast this spell, you may <part>"
// (CR 601.2b), printed S:Mode$ OptionalCost | EffectZone$ All | ValidCard$
// Card.Self | ValidSA$ Spell | Cost$ <part>. gorge (rules
// statics_costmods.go optionalCostViews, legal_walk_hand.go) offers each
// payable view as its own "optionalcost" cast beside the plain one, folds
// the part into the cost stages and stamps FlagOptionalCostPaid; Count$
// OptionalGenericCostPaid.<paid>.<unpaid> and Condition$ OptionalCost read
// that flag. The kernel's paid flag is the kicked provenance (sfKicked, as
// Bargain's), so only a part the kernel already pays as a cost lowers:
// Blight<N> (the pkBlight pick), Behold<1/Type> (the ChCostBehold pick) or
// CollectEvidence<N> (the bsEvidence pick; lower_collectedevidence.go) or
// Waterbend<N> (the kicked cast's {N} with the waterbend pick; kernel
// act_waterbend.go). Waterbend<X> and mana parts refuse by name.

// optCostStatic lowers the card's one OptionalCost static into lc.out.OptCost.
func (lc *lowerCtx) optCostStatic(s cards.Static) error {
	for _, k := range sortedParamKeys(s.Params) {
		switch k {
		case "Mode", "EffectZone", "ValidCard", "ValidSA", "Cost", "Description":
		default:
			return refuse("OptionalCost static with %s$", k)
		}
	}
	if strings.TrimSpace(s.Params["EffectZone"]) != "All" || strings.TrimSpace(s.Params["ValidCard"]) != "Card.Self" || strings.TrimSpace(s.Params["ValidSA"]) != "Spell" {
		return refuse("OptionalCost static shape EffectZone %q ValidCard %q ValidSA %q", s.Params["EffectZone"], s.Params["ValidCard"], s.Params["ValidSA"])
	}
	if lc.out.OptCost != nil {
		return refuse("a second OptionalCost static") // gorge offers one cast per view; the kernel has one kicked mode
	}
	if lc.out.Types&(tInstant|tSorcery) == 0 && !strings.HasPrefix(strings.TrimSpace(s.Params["Cost"]), "CollectEvidence<") {
		// sfKicked reaches a permanent's kicked provenance (mechKicked,
		// F2Kicked); only the evidence part is read there (lower_collectedevidence.go)
		return refuse("OptionalCost on a permanent spell")
	}
	o := lc.out
	if o.BeholdExile || o.Kicker != nil || o.Bargain || o.Offspring != nil || o.Gift || o.Casualty != 0 || o.Teamwork != 0 ||
		o.SpellCost != nil || o.AltAddCost != nil || o.Flashback != nil || o.Harmonize != nil || o.Mayhem != nil {
		return refuse("OptionalCost beside another optional or alternative cost")
	}
	raw := strings.TrimSpace(s.Params["Cost"])
	if strings.Contains(raw, "ChooseCreatureType<") {
		// a creature type chosen while paying, which the cost's behold and
		// the resolution then read (gorge cast_choose_type_cost.go): a
		// second mechanism the kernel's cost picks do not carry
		return refuse("OptionalCost cost %q: a creature type chosen as a cost part", raw)
	}
	lc.beholdOK = true
	var c lCost
	var err error
	if strings.HasPrefix(raw, "Waterbend<") {
		// costParts holds a waterbend to an activated ability's own cost; the
		// optional part is the kicked cast's (the checks below bound the rest)
		c, err = lc.costHand(raw)
	} else {
		c, err = lc.cost(raw)
	}
	lc.beholdOK = false
	if err != nil {
		return refuse("OptionalCost cost %q", raw)
	}
	rest := c
	rest.BlightN, rest.BeholdN, rest.BeholdF, rest.EvidenceN, rest.Waterbend = 0, 0, nil, 0, 0
	// lc.cost folds Waterbend<N>'s {N} into the mana (gorge ParseCost): that
	// generic is the part's own, and nothing else may be
	wbMana := c.Waterbend != 0 && c.Mana.Generic == c.Waterbend && c.Mana.Pips == [5]int{} && c.Mana.C == 0 &&
		len(c.Mana.Hybrid)+len(c.Mana.Twobrid)+len(c.Mana.Phyrexian) == 0 && !c.Mana.X && !c.Mana.None
	parts := 0
	for _, on := range []bool{c.BlightN != 0, c.BeholdN != 0, c.EvidenceN != 0, c.Waterbend != 0} {
		if on {
			parts++
		}
	}
	if !rest.pureMana() || !c.Mana.None && !wbMana || parts != 1 || c.BeholdN > 1 {
		return refuse("OptionalCost cost %q", raw)
	}
	lc.out.OptCost = &c
	return nil
}

// faceOptCost reports whether the face prints an OptionalCost static. The
// spell lines lower before the statics, so a paid read checks the face; a
// static that then refuses refuses the card.
func (lc *lowerCtx) faceOptCost() bool {
	for _, s := range lc.face.Statics {
		if s.Mode == "OptionalCost" {
			return true
		}
	}
	return false
}

// optCostRead gates Count$OptionalGenericCostPaid.<paid>.<unpaid> (gorge
// count_body_paid.go: the flag off the cast object): only in a spell's own
// chain (the kernel reads the resolving entry's sfKicked), only on an
// OptionalCost card, and with literal arms (an SVar arm is out of scope).
func (lc *lowerCtx) optCostRead(rest string) error {
	if !lc.faceOptCost() {
		return refuse("Count$%s without an OptionalCost static", rest)
	}
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return refuse("Count$OptionalGenericCostPaid outside a spell ability")
	}
	for _, p := range strings.Split(strings.TrimPrefix(rest, "OptionalGenericCostPaid."), ".") {
		if !isIntLit(p) {
			return refuse("Count$%s: a non-literal arm", rest)
		}
	}
	return nil
}

// reduceOptPaid lowers a self ReduceCost static whose Amount$ is
// Count$OptionalGenericCostPaid.<paid>.0 (Bite Down on Crime: "costs {2} less
// if evidence was collected"). gorge seeds the pending cast's optional-cost
// election while it prices the optional cast (rules costAmountCtx) and reads
// the unpaid arm for every other cast, so the static is the paid arm exactly
// on the optional-cost cast: a literal reduction that prices only that cast
// (flag OptPaid, the Bargain-gated static's carrier, kop.StBargained). The
// unpaid arm must be 0, since the plain cast then pays the full cost. Returns
// true when it read the Amount$.
func (lc *lowerCtx) reduceOptPaid(sp *params, st *lStatic) (bool, error) {
	body, ok := lc.face.SVars[strings.TrimSpace(sp.m["Amount"])]
	if !ok || !strings.HasPrefix(body, "Count$OptionalGenericCostPaid.") {
		return false, nil
	}
	sp.consume("Amount", "Relative") // gorge lists Relative$ as readable and never reads it
	if !lc.faceOptCost() {
		return true, refuse("Count$OptionalGenericCostPaid without an OptionalCost static")
	}
	if strings.TrimSpace(sp.m["ValidCard"]) != "Card.Self" || strings.TrimSpace(sp.m["EffectZone"]) != "All" {
		return true, refuse("ReduceCost reading Count$OptionalGenericCostPaid beside ValidCard %q EffectZone %q (only a self static with EffectZone$ All)", sp.m["ValidCard"], sp.m["EffectZone"])
	}
	parts := strings.Split(strings.TrimPrefix(body, "Count$OptionalGenericCostPaid."), ".")
	if len(parts) != 2 || !isIntLit(parts[0]) || parts[1] != "0" || parts[0] == "0" || len(parts[0]) > 2 {
		return true, refuse("ReduceCost reading %s (only a literal paid arm and an unpaid arm of 0)", body)
	}
	n, _ := strconv.Atoi(parts[0])
	st.Amount = cst(n)
	st.Flags = append(st.Flags, "OptPaid")
	return true, nil
}

// optCostCond lowers Condition$ OptionalCost (gorge conditions.go: the
// cast's FlagOptionalCostPaid): ConditionOptionalPaid$ True or absent reads
// paid (cKicked); False has no Std carrier and refuses.
func (lc *lowerCtx) optCostCond(ps *params) (lCond, error) {
	if !lc.faceOptCost() {
		return lCond{}, refuse("Condition$ OptionalCost without an OptionalCost static")
	}
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return lCond{}, refuse("Condition$ OptionalCost outside a spell")
	}
	if v, ok := ps.get("ConditionOptionalPaid"); ok && strings.TrimSpace(v) != "True" {
		return lCond{}, refuse("Condition$ OptionalCost with ConditionOptionalPaid %q", v)
	}
	return lCond{Kind: cKicked}, nil
}

func isIntLit(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
