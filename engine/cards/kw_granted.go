// The granted-keyword synthesizer (CR 613.1f). A layer-6 AddKeyword$ grant
// lands an EXPANDED keyword line in a face's DERIVED keyword list, but the
// link-time expansion (engine/cards/keywords.go expandKeywords) only ever reads a
// face's PRINTED keyword list, so no ability is minted for the grant. This
// file is the one dispatcher every granted-keyword read site calls: it
// synthesizes the ability body for every head whose printed expansion mints
// exactly one activated ability from the line alone: Cycling, TypeCycling,
// Saddle and Crew share a body builder with the printed expansion; Equip,
// Fortify, Level up, Outlast, Unearth, Embalm, Eternalize and Ninjutsu run the
// registered printed expander itself against a scratch face, so the two cannot
// drift. Reconfigure (a pair of abilities) and Transmute (the body reads the
// face's mana value) cannot be rebuilt from the line alone and stay absent.

package cards

import "strings"

// grantedKeywordHeads is the set of keyword heads GrantedKeywordAbility can
// synthesize an activated ability for. Every other head returns nil (fail
// closed): a keyword whose expansion needs more than the line (Reconfigure's
// attach/unattach pair, Transmute's printed mana value) would hand back a
// wrong body, which is worse than handing back none.
func grantedKeywordHeads(head string) bool {
	switch head {
	case "Cycling", "TypeCycling", "Saddle", "Crew":
		return true
	}
	return grantedExpanderHeads(head)
}

// grantedExpanderHeads is the subset of grantedKeywordHeads whose body is the
// printed expander's own single minted ability (grantedExpanderSA).
func grantedExpanderHeads(head string) bool {
	switch head {
	case "Equip", "Fortify", "Level up", "Outlast", "Unearth", "Embalm", "Eternalize", "Ninjutsu":
		return true
	}
	return false
}

// GrantedKeywordHeadNames lists every head GrantedKeywordAbility synthesizes,
// in a fixed order, for the rules-side head gates that must agree with it.
func GrantedKeywordHeadNames() []string {
	return []string{"Cycling", "TypeCycling", "Saddle", "Crew",
		"Equip", "Fortify", "Level up", "Outlast", "Unearth", "Embalm", "Eternalize", "Ninjutsu"}
}

// grantedExpanderSA runs the registered printed expander for line against a
// scratch face and returns the single ability it mints, tagged exactly as the
// printed expansion tags it (KeywordLine = line). A line whose expander mints
// nothing or more than one ability (a malformed cost) returns nil.
func grantedExpanderSA(line string) *SA {
	f := newFace()
	if !f.ApplyKeywordExpansion(line) || len(f.Abilities) != 1 {
		return nil
	}
	return f.Abilities[0]
}

// GrantedKeywordAbility synthesizes the activated ability a keyword LINE
// grants. The line is the full "Head:param..." text exactly as it sits in a
// face's keyword list ("Cycling:2", "Crew:1", "Saddle:2") or in a layer-6
// AddKeyword$ grant's derived entry ("Cycling:1 U", "TypeCycling:Sliver:3"):
// the same shape the printed expanders parse, so the granted body is
// byte-identical to the printed one's. The returned SA carries KeywordLine =
// the line, the same tag the printed expansion sets. A line whose head mints
// no activated ability -- or whose shape the construction cannot model --
// returns nil (the totality stance every synthesizer takes), so a caller
// fails closed to no ability rather than a wrong one.
func GrantedKeywordAbility(line string) *SA {
	head := KeywordHead(line)
	if !grantedKeywordHeads(head) {
		return nil
	}
	param := ""
	if j := strings.IndexByte(line, ':'); j >= 0 {
		param = strings.TrimSpace(line[j+1:])
	}
	var sa *SA
	switch head {
	case "Cycling":
		sa = cyclingAbilitySA(param)
	case "TypeCycling":
		// TypeCycling's param is "<type>:<cost>[...]" -- the same split the
		// printed expander (kwTypeCycling) runs.
		typeSpec, rest, _ := strings.Cut(param, ":")
		cost, _, _ := strings.Cut(rest, ":")
		sa = typeCyclingAbilitySA(strings.TrimSpace(typeSpec), strings.TrimSpace(cost))
	case "Saddle":
		// Saddle's param is "<N>[:...]"; a trailing field is display prose
		// and stays dropped, exactly as the printed expander drops it.
		sa = saddleAbilitySA(strings.TrimSpace(strings.Split(param, ":")[0]))
	case "Crew":
		n, riders := crewParamParts(param)
		sa = crewAbilitySA(n, riders)
	default:
		sa = grantedExpanderSA(line)
	}
	if sa != nil {
		sa.Params["KeywordLine"] = line
	}
	return sa
}
