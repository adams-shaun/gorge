package kgen

import "strings"

// Granted riot (CR 702.136a; gorge's enteringRiot reads the derived keyword
// list, so a static's AddKeyword$ Riot asks a creature's counter-or-haste
// election exactly as a printed K:Riot does). Riot has a CardDef field, not
// a KwMask bit, so the grant is its own kop.Static.Riot bool:
//
//   - a Continuous static's AddKeyword$ Riot (Spider-Punk's other Spiders,
//     Uncivil Unrest's and Rhythm of the Wild's nontoken creatures), a
//     layer-6 grant read beside the printed field by State.riotPermGranted /
//     State.riotCardGranted (dirac/kw_riot.go).
//
// An AddKeyword$ list may mix Riot with other terms; the rest still goes to
// the other splitters. More than one Riot term is one grant (the keyword is
// not cumulative).
func splitStaticRiotGrant(kw string) (rest string, riot bool) {
	var keep []string
	for _, k := range strings.Split(kw, "&") {
		if k = strings.TrimSpace(k); k == "Riot" {
			riot = true
			continue
		}
		keep = append(keep, k)
	}
	return strings.Join(keep, " & "), riot
}
