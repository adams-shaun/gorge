package kgen

import "strings"

// graveFbGrant lowers a Continuous static whose AffectedZone$ is Graveyard and
// which grants flashback to cards in its controller's graveyard (Iroh, Grand
// Lotus: non-Lesson instants and sorceries at their mana cost, Lessons at
// {1}, during your turn). gorge reads the granted keyword off the card's
// derived keywords (legal_walk_alt.go's flashback walk, cast_altcost.go
// flashbackCost), so the kernel reads the static live from the battlefield
// (kop.StatFbGrant, kw_fbgrant.go): no mark is stamped on the card.
//
// Only the controller's own graveyard is lowered (every Affected$ clause
// YouOwn, typed Instant, Sorcery or Lesson), from the battlefield, with a
// plain mana cost or none; any other card type, any other keyword, an EffectZone$ off the battlefield, an {X} or non-mana cost and a
// second grant beside it refuse by name.
func (lc *lowerCtx) graveFbGrant(sp *params, st *lStatic, aff string) error {
	kw := sp.str("AddKeyword")
	head, param, hasCost := strings.Cut(kw, ":")
	if head != "Flashback" || strings.Contains(kw, " & ") {
		return refuse("AffectedZone Graveyard grant of %q", kw)
	}
	if ez, ok := sp.get("EffectZone"); ok && strings.TrimSpace(ez) != "Battlefield" {
		return refuse("AffectedZone Graveyard flashback grant with EffectZone %q", ez)
	}
	for _, cl := range strings.Split(aff, ",") {
		if !strings.HasSuffix(cl, ".YouOwn") && !strings.HasSuffix(cl, "+YouOwn") {
			return refuse("AffectedZone Graveyard flashback grant to %q (not your graveyard)", cl)
		}
		if th, _, _ := strings.Cut(strings.TrimSpace(cl), "."); th != "Instant" && th != "Sorcery" && th != "Lesson" {
			return refuse("AffectedZone Graveyard flashback grant to %q (not an instant, sorcery or Lesson)", cl)
		}
	}
	for _, k := range []string{"AddPower", "AddToughness", "AddType", "RemoveKeyword", "AddAbility", "AddTrigger", "AddStaticAbility", "AddHiddenKeyword", "SetColor", "AddColor", "RemoveAllAbilities"} {
		if _, ok := sp.m[k]; ok {
			return refuse("AffectedZone Graveyard flashback grant beside %s$", k)
		}
	}
	if hasCost {
		mc, err := parseManaCost(param)
		if err != nil || mc.X || mc.None || strings.ContainsAny(param, ":<|") {
			return refuse("AffectedZone Graveyard flashback grant cost %q", param)
		}
		st.Cost = lCost{Mana: mc}
		st.Flags = append(st.Flags, "OwnCost")
	}
	f, err := lc.filter(aff)
	if err != nil {
		return err
	}
	st.F = f
	sp.consume("AddKeyword", "AffectedZone")
	st.Kind = "FbGrant"
	return nil
}
