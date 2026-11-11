package kgen

import "strings"

// splitLandwalkGrant takes the "Landwalk:<spec>" terms out of a Continuous
// static's AddKeyword$ list (Goblin King's "Landwalk:Mountain", Lord of
// Atlantis's "Landwalk:Island"): the rest of the list and one filter over the
// defending player's lands holding every term's clauses (each landwalk is its
// own ability, CR 702.14c, so a land matching any of them makes the attacker
// unblockable). gorge reads the derived keyword through
// combat.LandwalkEvades, which is blind to where the keyword came from; the
// kernel reads kop.Static.Landwalk the way it reads a printed
// CardDef.Landwalk (the spec's parameter up to a second colon).
//
// Only a global grant ("Other Goblins have mountainwalk") lowers: an
// Equipment's or Aura's grant (Affected$ ...EquippedBy / ...EnchantedBy) has
// no lockstep repro, so it stays refused.
func (lc *lowerCtx) splitLandwalkGrant(kw, affected string) (string, *lFilter, error) {
	var rest []string
	var out *lFilter
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		head, param, ok := strings.Cut(k, ":")
		if !ok || head != "Landwalk" {
			rest = append(rest, k)
			continue
		}
		if strings.Contains(affected, "Equipped") || strings.Contains(affected, "Enchanted") {
			return "", nil, refuse("keyword %q: a granted landwalk from an attached host", k)
		}
		spec, _, _ := strings.Cut(param, ":") // gorge LandwalkSpec: a second colon starts the description
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return "", nil, refuse("keyword %q: landwalk with no land spec", k)
		}
		f, err := lc.filter(spec)
		if err != nil {
			return "", nil, refuse("keyword %q: %v", k, err)
		}
		if out == nil {
			out = f
		} else {
			out = &lFilter{Clauses: append(append([]lClause(nil), out.Clauses...), f.Clauses...)}
		}
	}
	return strings.Join(rest, " & "), out, nil
}
