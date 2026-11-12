package kgen

import (
	"strconv"
	"strings"
)

// graveEscapeGrant lowers a Continuous static whose AffectedZone$ is Graveyard
// and which grants escape to cards in its controller's graveyard (Underworld
// Breach: "each nonland card in your graveyard has escape. The escape cost is
// equal to the card's mana cost plus exile three other cards from your
// graveyard"). gorge derives the keyword on the card while the static applies
// (legal_walk_alt.go's escape walk, altcast.go escapeCost), so the kernel
// reads the static live from the battlefield (kop.StatEscapeGrant,
// kw_escape.go): no mark is stamped on the card.
//
// Only the corpus shape is lowered: every Affected$ clause carries a YouOwn
// conjunct (the card's controller's own graveyard), the source is read from
// the battlefield, and the cost text is "CardManaCost
// ExileFromGrave<N/Card.Other/other>" (CR 702.42a: the card's mana cost plus
// N OTHER cards from the same graveyard; gorge exAsk skips the cast card
// itself). A grant with a condition, a different cost text or an EffectZone$
// off the battlefield refuses by name.
func (lc *lowerCtx) graveEscapeGrant(sp *params, st *lStatic, aff string) error {
	kw := sp.str("AddKeyword")
	head, param, hasCost := strings.Cut(kw, ":")
	if head != "Escape" || strings.Contains(kw, " & ") {
		return refuse("AffectedZone Graveyard grant of %q", kw)
	}
	if ez, ok := sp.get("EffectZone"); ok && strings.TrimSpace(ez) != "Battlefield" {
		return refuse("AffectedZone Graveyard escape grant with EffectZone %q", ez)
	}
	if _, ok := sp.get("Condition"); ok {
		return refuse("AffectedZone Graveyard escape grant with a Condition$")
	}
	for _, cl := range strings.Split(aff, ",") {
		own := false
		for _, part := range strings.Split(strings.TrimSpace(cl), "+") {
			// Underworld Breach: Card.YouOwn+nonLand (the typed head with the
			// you-own property on its own + part)
			if part == "YouOwn" || strings.HasSuffix(part, ".YouOwn") {
				own = true
			}
		}
		if !own {
			return refuse("AffectedZone Graveyard escape grant to %q (not your graveyard)", cl)
		}
	}
	for _, k := range []string{"AddPower", "AddToughness", "AddType", "RemoveKeyword", "AddAbility", "AddTrigger", "AddStaticAbility", "AddHiddenKeyword", "SetColor", "AddColor", "RemoveAllAbilities", "MayPlay", "Condition"} {
		if _, ok := sp.m[k]; ok {
			return refuse("AffectedZone Graveyard escape grant beside %s$", k)
		}
	}
	if !hasCost {
		return refuse("AffectedZone Graveyard escape grant with no cost")
	}
	// the cost text: the card's mana cost placeholder plus one
	// ExileFromGrave<N/Spec> part. The card's mana cost is not carried on the
	// static (the kernel prices it as any spell's); only the exile part is.
	parts := strings.Fields(param)
	if len(parts) != 2 || parts[0] != "CardManaCost" || !strings.HasPrefix(parts[1], "ExileFromGrave<") || !strings.HasSuffix(parts[1], ">") {
		return refuse("AffectedZone Graveyard escape grant cost %q", param)
	}
	spec := parts[1][len("ExileFromGrave<") : len(parts[1])-1]
	seg := strings.Split(spec, "/")
	if len(seg) != 2 && len(seg) != 3 {
		return refuse("AffectedZone Graveyard escape grant cost %q", param)
	}
	n, err := strconv.Atoi(seg[0])
	if err != nil || n < 1 || n > 15 {
		return refuse("AffectedZone Graveyard escape grant exile %q", seg[0])
	}
	// gorge cost.parse folds a cost spec's ";" alternation to ","
	// (Creature;Artifact: either type)
	f, err := lc.filter(strings.ReplaceAll(seg[1], ";", ","))
	if err != nil {
		return err
	}
	for _, cl := range f.Clauses {
		if cl.CmpX != "" {
			// gorge binds no X into the escape exile spec (CR 702.42a's
			// ExileFromGrave<X/...> corpus line is never offered)
			return refuse("AffectedZone Graveyard escape grant: a variable bound")
		}
	}
	st.Cost.ExileG, st.Cost.ExileGN = f, n
	af, err := lc.filter(aff)
	if err != nil {
		return err
	}
	st.F = af
	sp.consume("AddKeyword", "AffectedZone")
	st.Kind = "EscapeGrant"
	return nil
}

// inertCountAs reports whether a graveyard Continuous static's
// AddHiddenKeyword$ is the count-as pseudo-keyword the corpus's Pardic
// Firecat / Diligent Farmhand carry ("CARDNAME count as Flame Burst.", on
// themselves, so a Firecat in the graveyard counts as Flame Burst for Flame
// Burst's own count filter). gorge's printed Continuous static walk has no
// AddHiddenKeyword branch (layers.go), so the static registers nothing and is
// inert in both engines; the consumer card (Flame Burst's count reading the
// name) is a separate refusal.
func inertCountAs(hk, aff string) bool {
	return aff == "Card.Self" && strings.HasPrefix(hk, "CARDNAME count as ") && strings.HasSuffix(hk, ".")
}
