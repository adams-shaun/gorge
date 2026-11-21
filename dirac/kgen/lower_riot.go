package kgen

// kw:Riot (CR 702.136a; gorge engine/rules/cast_etbchoice.go faceETBChoice,
// replacement_etb.go applyRiotReplacement): "You may have this permanent enter
// with an additional +1/+1 counter on it. If you don't, it gains haste." The
// printed K:Riot is CardDef.Riot (kop.KwRiot): the kernel poses the
// counter-or-haste as-enters ask on every face-up battlefield entry
// (dirac/kw_riot.go).
//
// gorge reads the keyword off the PRINTED face only, so a GRANTED Riot (a
// static's AddKeyword$ Riot: Rhythm of the Wild, Uncivil Unrest, Spider-Punk's
// Spiders) poses no ask there; kgen keeps refusing that grant (kwMask has no
// bit for it) rather than lower a kernel behaviour gorge lacks.
//
// The ask is the face's first as-enters choice in gorge (riot, unleash, the
// read-ahead chapter, then the Repls); the kernel's pre-entry ask holds one
// pending election, so Riot beside Unleash or an as-enters choice is
// refused.

func init() {
	kwHandlers["Riot"] = (*lowerCtx).lowerRiot
}

// lowerRiot lowers a printed K:Riot to CardDef.Riot.
func (lc *lowerCtx) lowerRiot(node, param string) {
	if param != "" {
		lc.refuseNode(node, "riot with a parameter %q", param)
		return
	}
	lc.out.Riot = true
}

// refuseRiotCombos refuses Riot beside another as-enters choice (Unleash, an
// EntersAsChoice replacement) and on a multi-face card.
func (lc *lowerCtx) refuseRiotCombos() {
	out := lc.out
	if !out.Riot {
		return
	}
	if out.Unleash {
		lc.refuseNode("K:Riot", "Riot beside Unleash")
	}
	for _, r := range out.Repls {
		if r.Kind == "EntersAsChoice" {
			lc.refuseNode("K:Riot", "Riot beside an as-enters choice")
			break
		}
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Riot", "Riot on a multi-face card")
	}
}
