package kgen

import "strings"

// kw:Unleash (CR 702.86; gorge engine/rules/unleash.go): "You may have this
// permanent enter the battlefield with a +1/+1 counter on it. While it has a
// +1/+1 counter on it, it can't block." The printed K:Unleash is
// CardDef.Unleash (kop.KwUnleash): the kernel poses the as-enters counter ask
// on every face-up battlefield entry and puts one +1/+1 counter on "yes"
// (dirac/kw_unleash.go), and the block gate refuses a blocker carrying the
// keyword and at least one +1/+1 counter. A GRANTED Unleash (a static's
// AddKeyword$ Unleash, Tesak, Judith's Hellhound's other Dogs) is
// kop.Static.Unleash, read beside the printed field by the same ask and gate
// (splitStaticUnleashGrant below, dirac/kw_unleash.go): kwMask has no bit for
// Unleash, so the grant is its own static field, the way granted Riot is.
func (lc *lowerCtx) lowerUnleash(node string) {
	if lc.out.Riot {
		// a face that also prints Riot (lower_riot.go): gorge asks both
		// as-enters elections and the kernel's pre-entry ask carries one
		// kind
		lc.refuseNode(node, "a face that also prints Riot")
		return
	}
	lc.out.Unleash = true
}

// splitStaticUnleashGrant splits an Unleash grant out of an AddKeyword$ list
// (Tesak, Judith's Hellhound's Dog.Other+YouCtrl static): the keyword is not
// cumulative, so more than one Unleash term is one grant, and the other
// terms still go to the other splitters and pumpKeywords.
func splitStaticUnleashGrant(kw string) (rest string, unleash bool) {
	var keep []string
	for _, k := range strings.Split(kw, "&") {
		if k = strings.TrimSpace(k); k == "Unleash" {
			unleash = true
			continue
		}
		keep = append(keep, k)
	}
	return strings.Join(keep, " & "), unleash
}
