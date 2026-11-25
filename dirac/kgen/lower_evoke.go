package kgen

import (
	"strconv"
	"strings"
)

// K:Evoke:<cost> (CR 702.74a) and
// CastSA>Count$Adamant_<n>.<Colour>.<yes>.<no> (CR 702.5; gorge
// effects/count_body_heads.go evalAdamantBranch).

// refuseEvokeCombos refuses evoke beside any other cast cost route (the
// kernel's evoked cast pays one plain alternative mana cost of a
// single-face permanent card), on an {X} spell or on a multi-face card,
// the refuseWebSlingCombos shape.
func (lc *lowerCtx) refuseEvokeCombos() {
	out := lc.out
	if out.Evoke == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil || out.Dash != nil {
		lc.refuseNode("K:Evoke", "Evoke beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Evoke", "Evoke on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Evoke", "Evoke on a multi-face card")
	}
}

// evokePickCost lowers an evoke cost with a hand-exile part (Endurance's
// "ExileFromHand<1/Card.Green+Other/green card>", CR 702.74a, 118.9): the
// alternative cost's hand-exile shape (altPayLifeExileHand: ExileFromHand<1/
// Spec>, PayLife<1> beside it or not), whose filter it keeps (altCostCost),
// with any plain mana beside it as the evoke mana cost. The kernel's evoked
// cast pays the pick through its pick stage (CardDef.EvokePick).
func (lc *lowerCtx) evokePickCost(param string) error {
	c, err := lc.altCostCost(param)
	if err != nil {
		return err
	}
	if !altPayLifeExileHand(&c) {
		return refuse("evoke cost parts other than ExileFromHand<1/Spec>, alone or beside PayLife<1>")
	}
	mana := c.Mana
	if mana.None {
		mana = manaCost{}
	}
	if mana.X || len(mana.Twobrid) != 0 || len(mana.Phyrexian) != 0 {
		return refuse("evoke mana part %s", mana)
	}
	c.Mana = manaCost{None: true}
	lc.out.Evoke, lc.out.EvokePick = &mana, &c
	return nil
}

// adamantValue lowers the CastSA>Count$ / Count$ body rest
// ("Adamant_2.Red.2.0", or "Adamant.Red.2.0" for the keyword's standard
// three) at the three sites where the kernel reads the cast's spend
// exactly, all off the same stack-entry spend (stackEntry.spc):
//
//   - a self-ETB trigger's header (its CheckSVar$ intervening-if): bound as
//     the trigger is queued (kernel kw_evoke.go evokeEnter);
//   - a spell's own chain (Cauldron's Gift, Foreboding Fruit): the
//     resolving spell entry's spend (gorge adamantSpendVector reads the
//     spell object's ManaColorSpent; a copy was never cast and reads 0);
//   - a self ETB replacement's CheckSVar$ (K:etbCounter ... Adamant$, the
//     Throne of Eldraine Paladins): the resolving spell entering, the
//     Count$Converge precedent (value_converge.go); a permanent entering
//     any other way has no spell entry and reads 0.
//
// Anywhere else refuses.
func (lc *lowerCtx) adamantValue(rest string) (lValue, error) {
	spell := lc.ab != nil && lc.ab.Kind == akSpell && !lc.ab.Latent && lc.ab.Trigger == nil
	repl := lc.replSelf && !lc.cloneRepl
	if !spell && !repl && lc.adamantTrigger() == nil {
		return lValue{}, refuse("Count$%s outside a spell, its self ETB replacement or a self-ETB trigger's CheckSVar$", rest)
	}
	n, tail := 3, ""
	if t, ok := strings.CutPrefix(rest, "Adamant_"); ok {
		ns, tl, found := strings.Cut(t, ".")
		v, err := strconv.Atoi(strings.TrimSpace(ns))
		if !found || err != nil {
			return lValue{}, refuse("CastSA>Count$%s", rest)
		}
		n, tail = v, tl
	} else if t, ok := strings.CutPrefix(rest, "Adamant."); ok {
		tail = t
	} else {
		return lValue{}, refuse("CastSA>Count$%s", rest)
	}
	if n < 1 || n > 3 {
		return lValue{}, refuse("CastSA>Count$%s threshold %d (the kernel's per-colour spend counts saturate at 3)", rest, n)
	}
	parts := strings.Split(tail, ".")
	if len(parts) != 3 {
		return lValue{}, refuse("CastSA>Count$%s arms", rest)
	}
	ci := strings.Index("WUBRG", adamantColourLetter(parts[0]))
	if ci < 0 || adamantColourLetter(parts[0]) == "" {
		return lValue{}, refuse("CastSA>Count$%s colour %q", rest, parts[0])
	}
	var arms [2]lValue
	for i, p := range parts[1:] {
		a, err := strconv.Atoi(p)
		if err != nil {
			return lValue{}, refuse("CastSA>Count$%s arm %q (an SVar arm)", rest, p)
		}
		arms[i] = cst(a)
	}
	return lValue{Kind: vAdamant, N: ci | n<<3, Sub: []lValue{arms[0], arms[1]}}, nil
}

// trigManaSpendCond lowers a trigger's ManaSpent$ / ManaNotSpent$ colour
// ("sacrifice it unless {W} was spent to cast it", CR 601.2h; gorge
// trigger_condition.go manaColourSpendHolds) to the same Count$Adamant read
// a self-ETB trigger's CheckSVar$ lowers to: the kernel binds the entering
// spell's per-colour spend into the pending trigger as it queues and the
// intervening-if recheck reads the bound count (kw_evoke.go exAdamant), the
// snapshot-or-live rule gorge's read follows. `not` picks the arms: the
// gate holds while the colour was NOT spent (the sacrifice arm) or while it
// was. Only the corpus spellings lower: one colour name or letter, on the
// card's own ETB trigger; a colour list, colourless, or any other trigger
// refuses.
func (lc *lowerCtx) trigManaSpendCond(colours string, not bool) (lCond, error) {
	if lc.adamantTrigger() == nil {
		return lCond{}, refuse("trigger ManaSpend outside the card's own ETB trigger")
	}
	if letter := adamantColourLetter(colours); letter == "" || len(strings.Fields(colours)) != 1 {
		return lCond{}, refuse("trigger ManaSpend colours %q", colours)
	}
	ci := strings.Index("WUBRG", adamantColourLetter(colours))
	yes, no := cst(1), cst(0)
	if not {
		yes, no = cst(0), cst(1)
	}
	return lCond{Kind: cValue, A: lValue{Kind: vAdamant, N: ci | 1<<3, Sub: []lValue{yes, no}}, Op: opEQ, B: cst(1)}, nil
}

// adamantTrigger is the trigger whose header condition is being lowered, if
// it is an ETB of the card itself (nil otherwise).
func (lc *lowerCtx) adamantTrigger() *lTrigger {
	if lc.ab == nil || lc.ab.Trigger == nil || lc.trigBody || lc.ab.Latent {
		return nil
	}
	tr := lc.ab.Trigger
	if tr.Event != "ETB" || tr.Batch || tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) != 1 || tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
		return nil
	}
	return tr
}

// adamantColourLetter is the WUBRG letter of an Adamant colour token (a
// colour name or its letter), "" for anything else (colorless, any).
func adamantColourLetter(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "white", "w":
		return "W"
	case "blue", "u":
		return "U"
	case "black", "b":
		return "B"
	case "red", "r":
		return "R"
	case "green", "g":
		return "G"
	}
	return ""
}
