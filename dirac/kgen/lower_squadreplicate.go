package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// K:Squad / K:Replicate (CR 702.157a / 702.56a): "as an additional cost to
// cast this spell, you may pay [cost] any number of times". Both are the
// multikicker shape (lower.go's K:Multikicker arm): the keyword's parameter
// is one payment, the kernel's kicked cast offers base plus one payment and
// then asks a count (kw_multikicker.go's bsMulti), and the count rides the
// same cold.mkObj/cold.mk storage. The two differences are the ask's stage
// in gorge's cast announcement (Replicate before the multikicker ask, Squad
// after it; cast_begin.go 1154/1161 and chorus_cost.go preCostAsks) and the
// non-mana pick parts the corpus prints (Exterminate!'s tapXType, Ruthless
// Radrat's ExileFromGrave, Thrill-Kill Disciple's Discard, Reiterating
// Bolt's PayEnergy). RepeatedKind records which keyword owns the slot and
// Repeated the full payment; Ms Multikicker slot carries its mana half.

func init() {
	kwHandlers["Squad"] = (*lowerCtx).lowerSquad
	kwHandlers["Replicate"] = (*lowerCtx).lowerReplicate
	kwHandlers["Conspire"] = (*lowerCtx).lowerConspire
}

func (lc *lowerCtx) lowerSquad(node, param string) {
	lc.lowerRepeated(node, param, kop.RepeatSquad, "squad")
}

func (lc *lowerCtx) lowerReplicate(node, param string) {
	lc.lowerRepeated(node, param, kop.RepeatReplicate, "replicate")
}

// lowerRepeated lowers one repeated additional cost. Two of the three
// keywords on one card refuse by name: the kernel's kicked cast carries one
// count in the shared slot.
func (lc *lowerCtx) lowerRepeated(node, param string, kind uint8, word string) {
	if lc.out.RepeatedKind != 0 {
		lc.refuseNode(node, "two repeated additional costs")
		return
	}
	if strings.TrimSpace(param) == "" {
		lc.refuseNode(node, "%s cost %q", word, param)
		return
	}
	c, err := lc.cost(param)
	if err != nil || !repeatableCost(c) {
		lc.refuseNode(node, "%s cost %q", word, param)
		return
	}
	mc := c.Mana
	lc.out.Multikicker = &mc
	lc.out.Repeated = &c
	lc.out.RepeatedKind = kind
}

// lowerMultikickerKeyword is lowerKeywords' K:Multikicker arm, moved here
// beside the other two repeated-additional-cost keywords so the one slot's
// rules (and the "two repeated additional costs" refusal) have one home.
func (lc *lowerCtx) lowerMultikickerKeyword(node, param string) {
	// CR 702.43: the per-payment additional cost, paid any number of times as
	// the spell is cast. The kernel's kicked cast asks the count before X and
	// the targets (multikick, cast.go), so only a plain mana parameter on a
	// non-{X} spell is modelled; a non-mana, {X} parameter or {X} base cost
	// refuses (the count bound would be read before the X announcement).
	if lc.out.RepeatedKind != 0 {
		lc.refuseNode(node, "two repeated additional costs")
		return
	}
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.Contains(param, ":") || mc.None || mc.X {
		lc.refuseNode(node, "multikicker cost %q", param)
		return
	}
	if lc.out.Cost.X {
		lc.refuseNode(node, "multikicker on an {X} spell")
		return
	}
	lc.out.Multikicker = &mc
	lc.out.RepeatedKind = kop.RepeatMultikicker
}

// repeated copies a card's repeated-payment slot onto its CardDef: the mana
// half to Multikicker (so the kernel's kicked offer/build cost/count bound
// are shared with K:Multikicker) and the full payment to Repeated.
func (cb *cardBuilder) repeated(d *kop.CardDef, c *lCard) error {
	d.HasMultikicker = true
	d.RepeatedKind = c.RepeatedKind
	var err error
	if d.Multikicker, err = manaCostOf(*c.Multikicker); err != nil {
		return err
	}
	if c.Repeated != nil {
		if d.Repeated, err = cb.cost(*c.Repeated); err != nil {
			return err
		}
	}
	return nil
}

// repeatableCost reports whether the parsed cost is one the kernel's
// repeated payment can scale: mana plus at most one counted part whose
// payment is N identical picks (a discard, a graveyard exile, a tap or
// energy). Cost.N then multiplies by the answered count, exactly the sum of
// the same part paid once per payment, which is gorge's composed cost. A
// part with an announced X, the source itself, a filter, a second part or a
// kind the picks cannot repeat stays refused by the caller.
func repeatableCost(c lCost) bool {
	if c.Mana.X {
		return false
	}
	if c.pureMana() {
		return true
	}
	rest := c
	switch {
	case c.DiscardN > 0 && c.DiscardSpec == "" && c.DiscardF == nil && !c.DiscardX && c.Discard2N == 0:
		rest.DiscardN = 0
	case c.ExileG != nil && !c.ExileGX && !c.ExileGSelf && !c.ExileGBf && c.ExileGN > 0 && c.ExileHandN == 0 && !c.ExileHand:
		rest.ExileG, rest.ExileGN = nil, 0
	case c.TapN > 0 && c.TapF != nil && !c.TapXVar:
		rest.TapN, rest.TapF = 0, nil
	case c.Energy > 0 && c.EnergyBy == nil:
		rest.Energy = 0
	default:
		return false
	}
	return rest.pureMana()
}

// repeatedPaidValue lowers Count$SquadPaid / Count$ReplicatePaid /
// Count$Conspired to the paid count the kernel stores per spell
// (vPaidN, dirac/cost_paidcount.go), but only on a card whose own keyword is the
// matching one: anywhere else the SVar has no cast-time payment behind it in
// the kernel.
func (lc *lowerCtx) repeatedPaidValue(rest string) (lValue, bool, error) {
	ok := false
	switch rest {
	case "SquadPaid":
		ok = lc.out.RepeatedKind == kop.RepeatSquad
	case "ReplicatePaid":
		ok = lc.out.RepeatedKind == kop.RepeatReplicate
	case "Conspired":
		ok = lc.out.Conspire
	}
	if !ok {
		return lValue{}, true, refuse("Count$%s without the matching keyword", rest)
	}
	return lValue{Kind: vPaidN}, true, nil
}

// lowerConspire is printed K:Conspire (CR 702.78a): the conspired cast and its
// tap election are the kernel's (dirac/kw_conspire.go, CardDef.Conspire), the
// copy is gorge kwConspire's SpellCast Card.Self trigger lowered from its own
// line (CheckSVar$ Count$Conspired, CopySpellAbility Amount$ Count$Conspired).
// The conspired cast is offered beside the plain hand cast only, so
// refuseConspireCombos refuses a card with another cast route, an additional
// or optional cost, or a second face by name.
func (lc *lowerCtx) lowerConspire(node, param string) {
	switch {
	case strings.TrimSpace(param) != "":
		lc.refuseNode(node, "conspire parameter %q", param)
	case lc.out.Conspire:
		lc.refuseNode(node, "second Conspire keyword")
	default:
		lc.out.Conspire = true
	}
}

// refuseConspireCombos refuses a Conspire card beside any other cast route or
// cost the conspired cast does not carry (run after every line has lowered,
// as refuseRepeatedCombos is).
func (lc *lowerCtx) refuseConspireCombos() {
	out := lc.out
	if !out.Conspire {
		return
	}
	if out.RepeatedKind != 0 || out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil ||
		out.AltAddCost != nil || out.Sneak != nil || out.Impending != nil || out.Bestow != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil ||
		out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree ||
		out.Convoke || out.Improvise || out.Storm || out.Buyback != nil || out.Entwine != nil ||
		len(lc.card.Faces) > 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Conspire", "Conspire beside another cast route or additional cost")
		return
	}
	if out.Cost.X {
		lc.refuseNode("K:Conspire", "Conspire on an {X} spell")
	}
}

// refuseRepeatedCombos refuses Squad/Replicate beside any other cast route:
// the kernel's kicked cast carries the repeated payment as its one extra
// cost (the refuseMultikickerCombos list), and the count is asked before an
// {X} base cost is announced, so an {X} spell cannot bound it.
func (lc *lowerCtx) refuseRepeatedCombos() {
	out := lc.out
	if out.Repeated == nil {
		return
	}
	node, word := "K:Squad", "Squad"
	if out.RepeatedKind == kop.RepeatReplicate {
		node, word = "K:Replicate", "Replicate"
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil ||
		out.AltAddCost != nil || out.Sneak != nil || out.Impending != nil || out.Bestow != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil ||
		out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree ||
		out.Convoke || out.Improvise || len(lc.card.Faces) > 1 || lc.card.AlternateMode != "" {
		lc.refuseNode(node, "%s beside another cast route or additional cost", word)
		return
	}
	if out.Cost.X {
		lc.refuseNode(node, "%s on an {X} spell", word)
	}
}
