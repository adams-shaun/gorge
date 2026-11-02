package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// grantedLookOnly: AddStaticAbility$ naming one SVar
// whose body is the exact view-only library-top look L1 lowers to nothing
// (Glowcap Lantern). gorge grants it to each host (rules/layers.go:630-660);
// it has no event, decision or legality reader, so the grant lowers to
// nothing and the param is consumed. Any other body leaves the param
// unread (the static's unread check refuses it by name).
func (lc *lowerCtx) grantedLookOnly(sp *params) error {
	v, ok := sp.m["AddStaticAbility"]
	if !ok {
		return nil
	}
	name := strings.TrimSpace(v)
	if strings.ContainsAny(name, ",&") {
		return nil
	}
	body, ok := lc.face.SVars[name]
	if !ok {
		return nil
	}
	sts, ok := cards.ParseStaticLines(body)
	if !ok || len(sts) != 1 {
		return nil
	}
	in := sts[0]
	for k := range in.Params {
		switch k {
		case "Mode", "Affected", "AffectedZone", "MayLookAt", "Description":
		default:
			return nil
		}
	}
	aff := strings.TrimSpace(in.Params["Affected"])
	ml := strings.TrimSpace(in.Params["MayLookAt"])
	if in.Mode != "Continuous" || strings.TrimSpace(in.Params["AffectedZone"]) != "Library" ||
		aff != "Card.TopLibrary+YouCtrl" && aff != "Card.TopLibrary+YouOwn" ||
		ml != "You" && ml != "True" && ml != "Player" {
		return nil
	}
	sp.consume("AddStaticAbility")
	return nil
}
