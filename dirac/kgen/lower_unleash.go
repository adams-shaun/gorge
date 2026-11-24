package kgen

// kw:Unleash (CR 702.86; gorge engine/rules/unleash.go): "You may have this
// permanent enter the battlefield with a +1/+1 counter on it. While it has a
// +1/+1 counter on it, it can't block." The printed K:Unleash is
// CardDef.Unleash (kop.KwUnleash): the kernel poses the as-enters counter ask
// on every face-up battlefield entry and puts one +1/+1 counter on "yes"
// (dirac/kw_unleash.go), and the block gate refuses a blocker carrying the
// keyword and at least one +1/+1 counter. A GRANTED Unleash (a static's
// AddKeyword$ Unleash, Tesak, Judith's Hellhound) stays refused: kwMask has
// no bit for it and the kernel reads only the printed field.
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
