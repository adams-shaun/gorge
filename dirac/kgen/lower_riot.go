package kgen

func init() { kwHandlers["Riot"] = (*lowerCtx).lowerRiotKw }

// lowerRiotKw lowers a printed bare K:Riot (CR 702.136a): the printed keyword
// is CardDef.Riot (kop.KwRiot), and the kernel poses the as-enters
// counter-or-haste election on every face-up battlefield entry from it
// (dirac/kw_riot.go: a cast creature's election rides the pre-entry ask, a
// non-cast entry queues the ask; "counter" places one +1/+1 counter the way
// the entry counters are put, "haste" grants the keyword the way gorge's
// IntrinsicKeywords append does). Gorge reads the printed line directly
// (cast_etbchoice.go's faceETBChoice, enteringRiot), so no expansion line
// belongs to the head; the printed keyword stays on the battlefield and a
// battlefield re-entry asks again (CR 400.7's new object asks too). A face
// that also prints Unleash refuses: gorge asks both as-enters elections and
// the kernel's pre-entry ask carries one kind (no corpus card has both). A
// GRANTED Riot (a static's AddKeyword$, Rhythm of the Wild, Spider-Punk)
// stays refused: gorge's enteringRiot would ask for the derived read too,
// and the kernel reads only the printed field.
func (lc *lowerCtx) lowerRiotKw(node, _ string) {
	if lc.out.Unleash {
		lc.refuseNode(node, "a face that also prints Unleash")
		return
	}
	lc.out.Riot = true
}
