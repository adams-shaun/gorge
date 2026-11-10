package kgen

import "strings"

// handWarpGrant lowers a Continuous static whose AffectedZone$ is Hand and
// which grants warp to cards in its controller's hand (Tannuk, Steadfast
// Second: "Artifact cards and red creature cards in your hand have warp
// {2}{R}", CR 702.185a). gorge reads the granted keyword off the card's
// derived keywords (rules warp_grant.go grantedWarpCosts), so the kernel
// reads the static live from the battlefield (kop.StatWarpGrant,
// kw_warpgrant.go): no mark is stamped on the card.
//
// Only the controller's own hand is lowered (every Affected$ clause YouOwn,
// typed Artifact or Creature), from the battlefield, unconditionally, with a
// plain mana warp cost; any other card type, an EffectZone$ off the
// battlefield, a condition, an {X}, hybrid or non-mana cost and anything
// beside the grant refuse by name.
func (lc *lowerCtx) handWarpGrant(sp *params, st *lStatic, aff string) error {
	kw := sp.str("AddKeyword")
	head, param, hasCost := strings.Cut(kw, ":")
	if head != "Warp" || !hasCost || strings.Contains(kw, " & ") {
		return refuse("AffectedZone Hand grant of %q", kw)
	}
	if ez, ok := sp.get("EffectZone"); ok && strings.TrimSpace(ez) != "Battlefield" {
		return refuse("AffectedZone Hand warp grant with EffectZone %q", ez)
	}
	if st.Cond.Kind != cNone {
		return refuse("AffectedZone Hand warp grant with a condition")
	}
	for _, cl := range strings.Split(aff, ",") {
		if !strings.HasSuffix(cl, ".YouOwn") && !strings.HasSuffix(cl, "+YouOwn") {
			return refuse("AffectedZone Hand warp grant to %q (not your hand)", cl)
		}
		if th, _, _ := strings.Cut(strings.TrimSpace(cl), "."); th != "Artifact" && th != "Creature" {
			return refuse("AffectedZone Hand warp grant to %q (not an artifact or creature)", cl)
		}
	}
	for _, k := range []string{"AddPower", "AddToughness", "AddType", "RemoveKeyword", "AddAbility", "AddTrigger", "AddStaticAbility", "AddHiddenKeyword", "SetColor", "AddColor", "RemoveAllAbilities"} {
		if _, ok := sp.m[k]; ok {
			return refuse("AffectedZone Hand warp grant beside %s$", k)
		}
	}
	mc, err := parseManaCost(param)
	if err != nil || mc.X || mc.None || len(mc.Hybrid) != 0 || len(mc.Twobrid) != 0 || len(mc.Phyrexian) != 0 || strings.ContainsAny(param, ":<|") {
		return refuse("AffectedZone Hand warp grant cost %q", param)
	}
	f, err := lc.filter(aff)
	if err != nil {
		return err
	}
	st.F = f
	st.Cost = lCost{Mana: mc}
	sp.consume("AddKeyword", "AffectedZone")
	st.Kind = "WarpGrant"
	return nil
}
