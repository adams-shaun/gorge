package kgen

import "strings"

// madnessGrantZones are the AffectedZone$ words a granted Madness lists
// (Falkenrath Gorger: "Hand,Library,Graveyard,Exile,Command,Stack"), in the
// order the lowered statics are emitted.
var madnessGrantZones = map[string]int{
	"Hand": zHand, "Library": zLibrary, "Graveyard": zGraveyard,
	"Exile": zExile, "Command": zCommand, "Stack": zStack,
}

// madnessGrant lowers a Continuous static that grants madness to the cards in
// the zones its AffectedZone$ lists (Falkenrath Gorger: "Each Vampire creature
// card you own that isn't on the battlefield has madness. The madness cost is
// equal to its mana cost", CR 702.35a-b). gorge reads the granted keyword off
// the card's derived keywords (rules altcast.go madnessReplacementApplies,
// offerMadness, madnessCostLabel), at the discard, the resolution ask and the
// cast, so the kernel reads the static live from the battlefield
// (kop.StatMadnessGrant, kw_madness.go madnessCostOf): no mark is stamped on
// the card. One static is emitted per listed zone (Static.Zone), each over the
// same filter.
//
// The parameter is "ManaCost" or "ManaCost:<N>" (the card's own cost, less N
// generic: CardCostLess is N+1), as the granted miracle's is. A literal cost, a
// cost with an {X}, a hybrid or Phyrexian pip or a coloured reduction, an
// unknown, repeated, Battlefield or All zone, an EffectZone$ off the
// battlefield and anything beside the grant refuse by name.
func (lc *lowerCtx) madnessGrant(sp *params, st *lStatic, aff string) ([]lStatic, error) {
	kw := strings.TrimSpace(sp.str("AddKeyword"))
	head, param, hasParam := strings.Cut(kw, ":")
	if head != "Madness" || !hasParam || strings.Contains(kw, "&") {
		return nil, refuse("AffectedZone %q grant of %q", sp.str("AffectedZone"), kw)
	}
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "AffectedZone", "AddKeyword", "Description", "Condition", "Secondary":
		case "EffectZone":
			if strings.TrimSpace(sp.str("EffectZone")) != "Battlefield" {
				return nil, refuse("madness grant with EffectZone %q", sp.str("EffectZone"))
			}
		default:
			return nil, refuse("madness grant beside %s$", k)
		}
	}
	var zones []int
	seen := map[int]bool{}
	for _, w := range strings.Split(sp.str("AffectedZone"), ",") {
		z, ok := madnessGrantZones[strings.TrimSpace(w)]
		if !ok {
			return nil, refuse("madness grant over AffectedZone %q", strings.TrimSpace(w))
		}
		if seen[z] {
			return nil, refuse("madness grant lists AffectedZone %q twice", strings.TrimSpace(w))
		}
		seen[z] = true
		zones = append(zones, z)
	}
	param = strings.TrimSpace(param)
	rest, ok := strings.CutPrefix(param, "ManaCost")
	if !ok {
		return nil, refuse("madness grant cost %q", param)
	}
	less := 0
	if rest != "" {
		n, ok := strings.CutPrefix(rest, ":")
		mc, err := parseManaCost(n)
		if !ok || err != nil || mc.Generic == 0 || mc.Pips != [5]int{} || mc.C != 0 || mc.X || mc.XX || mc.None ||
			len(mc.Hybrid) != 0 || len(mc.Twobrid) != 0 || len(mc.Phyrexian) != 0 || mc.Generic > 250 {
			return nil, refuse("madness grant cost %q", param)
		}
		less = mc.Generic
	}
	f, err := lc.filter(aff)
	if err != nil {
		return nil, err
	}
	if f != nil && f.Players != 0 {
		return nil, refuse("madness grant to a player-spec Affected$ %q", aff)
	}
	sp.consume("AddKeyword", "AffectedZone", "EffectZone")
	st.F = f
	st.Kind = "MadnessGrant"
	st.CardCostLess = less + 1
	out := make([]lStatic, 0, len(zones))
	for _, z := range zones {
		s := *st
		s.Zone = z
		out = append(out, s)
	}
	return out, nil
}
