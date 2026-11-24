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
// A global grant ("Other Goblins have mountainwalk"), an Equipment's
// (Trailblazer's Boots: "Equipped creature has nonbasic landwalk") and an
// Aura's (Burrowing: "Enchanted creature has mountainwalk") lower; the
// equipped or enchanted creature is the static's filter (Affected$
// ...EquippedBy / ...EnchantedBy), so the kernel reads the grant the way it
// reads a global one. A creature's own grant that names an Equipment form
// (the Equipped filter on a non-artifact) stays refused.
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
		if strings.Contains(affected, "Equipped") && lc.out.Types&tArtifact == 0 {
			return "", nil, refuse("keyword %q: a granted landwalk from an attached host", k)
		}
		f, err := lc.landwalkFilter(k, grantLandSpec(param))
		if err != nil {
			return "", nil, err
		}
		if out == nil {
			out = f
		} else {
			out = &lFilter{Clauses: append(append([]lClause(nil), out.Clauses...), f.Clauses...)}
		}
	}
	return strings.Join(rest, " & "), out, nil
}

// grantLandSpec rewrites a static grant's bare "ChosenType" land spec
// (Traveler's Cloak's "Landwalk:ChosenType", landwalk of the type the HOST
// chose) to the land filter "Land.ChosenType"; the kernel reads it against the
// static's host (dirac landwalkGranted), gorge resolves it against the host
// where the keyword is built (effects.ResolveGrantedLandwalk). Any other param
// passes through. A Pump's KW$ list keeps the bare spec, which stays refused.
func grantLandSpec(param string) string {
	spec, rest, more := strings.Cut(param, ":")
	if strings.TrimSpace(spec) != "ChosenType" {
		return param
	}
	if more {
		return "Land.ChosenType:" + rest
	}
	return "Land.ChosenType"
}

// landwalkFilter lowers one "Landwalk:<spec>" keyword term's land filter (k is
// the whole term, param what follows its first colon): the spec up to a second
// colon, which gorge's LandwalkSpec reads and starts the description at.
func (lc *lowerCtx) landwalkFilter(k, param string) (*lFilter, error) {
	spec, _, _ := strings.Cut(param, ":")
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, refuse("keyword %q: landwalk with no land spec", k)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return nil, refuse("keyword %q: %v", k, err)
	}
	return f, nil
}

// splitLandwalkPump takes the "Landwalk:<spec>" terms out of a Pump's KW$ list
// (Piracy Charm's "target creature gains islandwalk until end of turn"): the
// rest of the list and one filter holding every term's clauses, as
// splitLandwalkGrant builds it. pumpBody applies it as an end-of-turn override
// record carrying the filter (kop.Override.Landwalk), which the kernel reads
// beside a printed or statically granted landwalk.
func (lc *lowerCtx) splitLandwalkPump(kw string) (string, *lFilter, error) {
	var rest []string
	var out *lFilter
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		head, param, ok := strings.Cut(k, ":")
		if !ok || head != "Landwalk" {
			rest = append(rest, k)
			continue
		}
		f, err := lc.landwalkFilter(k, param)
		if err != nil {
			return "", nil, err
		}
		if out == nil {
			out = f
		} else {
			out = &lFilter{Clauses: append(append([]lClause(nil), out.Clauses...), f.Clauses...)}
		}
	}
	return strings.Join(rest, " & "), out, nil
}
