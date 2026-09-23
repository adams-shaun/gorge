// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwCycling(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	if sa := cyclingAbilitySA(param); sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

// cyclingAbilitySA builds the plain cycling ability body (CR 702.29a) the
// printed K:Cycling line expands to. It is shared by the printed expansion
// (kwCycling) and the granted route (GrantedCyclingAbility) so the two
// constructions cannot drift.
func cyclingAbilitySA(cost string) *SA {
	sa, _ := parseSA("", "AB$ Draw | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | NumCards$ 1 | Keyword$ Cycling | SpellDescription$ Cycling "+cost)
	return sa
}

// GrantedCyclingAbility synthesizes the activated ability a cycling keyword
// LINE grants. The line is the full "Head:param..." text exactly as it sits
// in a face's keyword list ("Cycling:2", "Cycling:1 U") or in a layer-6
// AddKeyword$ grant's derived entry ("Cycling:1 U", "TypeCycling:Sliver:3"):
// the same shape the printed expanders parse, so the granted body is
// byte-identical to the printed one's. The returned SA carries KeywordLine =
// the line, the same tag the printed expansion sets. A line whose head is
// neither Cycling nor TypeCycling -- or whose shape the construction cannot
// model -- returns nil (the totality stance every synthesizer takes), so a
// caller fails closed to no ability rather than a wrong one.
func GrantedCyclingAbility(line string) *SA {
	head := KeywordHead(line)
	param := ""
	if j := strings.IndexByte(line, ':'); j >= 0 {
		param = strings.TrimSpace(line[j+1:])
	}
	switch head {
	case "Cycling":
		sa := cyclingAbilitySA(param)
		if sa != nil {
			sa.Params["KeywordLine"] = line
		}
		return sa
	case "TypeCycling":
		// TypeCycling's param is "<type>:<cost>[...]" -- the same split the
		// printed expander (kwTypeCycling) runs.
		typeSpec, rest, _ := strings.Cut(param, ":")
		cost, _, _ := strings.Cut(rest, ":")
		sa := typeCyclingAbilitySA(strings.TrimSpace(typeSpec), strings.TrimSpace(cost))
		if sa != nil {
			sa.Params["KeywordLine"] = line
		}
		return sa
	}
	return nil
}

func init() { registerKeyword(kwCycling, "Cycling") }
