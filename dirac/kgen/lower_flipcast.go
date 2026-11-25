package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() {
	kwHandlers["More Than Meets the Eye"] = func(lc *lowerCtx, node, param string) { lc.lowerFlipCastKw(node, param, kop.FlipConverted) }
	kwHandlers["Disturb"] = func(lc *lowerCtx, node, param string) { lc.lowerFlipCastKw(node, param, kop.FlipDisturb) }
	kwHandlers["Living metal"] = (*lowerCtx).lowerLivingMetalKw
}

// lowerFlipCastKw lowers a front face's back-face cast keyword: K:More Than
// Meets the Eye:<cost> (CR 702.162a; gorge rules mtmteCast, the hand's
// "converted" cast) or K:Disturb:<cost> (CR 702.146a; gorge disturbCast, the
// graveyard's "disturb" cast). gorge reads the keyword's cost straight off
// the front face (rules cast_altcost.go mtmteCost / disturbCost), so it has
// no expansion line; the kernel casts face 1 at CardDef.FlipCost
// (dirac/face_flipcast.go). Only a plain mana cost on the front of a
// two-face transforming card, not a token, whose back is neither a land nor
// a Battle, lowers; X, hybrid and non-mana costs refuse, as does a second
// instance.
func (lc *lowerCtx) lowerFlipCastKw(node, param string, kind uint8) {
	c := lc.card
	switch {
	case lc.out.Token || faceLayout(c, false) != kop.FaceTransform || lc.face != c.Faces[0]:
		lc.refuseNode(node, "back-face cast keyword off the front of a transforming card")
		return
	case c.Faces[1].IsLand() || faceIsBattle(c.Faces[0]) || faceIsBattle(c.Faces[1]):
		lc.refuseNode(node, "back-face cast keyword with a land or Battle face")
		return
	case lc.out.FlipCost != nil:
		lc.refuseNode(node, "a second back-face cast keyword")
		return
	}
	mc, err := parseManaCost(param)
	if err != nil || param == "" || strings.ContainsAny(param, ":<") || mc.X || mc.None || len(mc.Hybrid) != 0 {
		lc.refuseNode(node, "back-face cast cost %q", param)
		return
	}
	lc.out.FlipCast, lc.out.FlipCost = kind, &mc
}

// lowerLivingMetalKw lowers K:Living metal (CR 702.161a): gorge's
// kwLivingMetal (cards/kw_livingmetal.go) expands it into one layer-4
// Continuous static on the permanent itself (AddType$ Artifact & Creature,
// Condition$ PlayerTurn), lowered from that line as the face's own Override.
// The head is a record for the with-keyword word (CardDef.LivingMetal). A
// face whose expansion a printed S: line suppressed refuses.
func (lc *lowerCtx) lowerLivingMetalKw(node, param string) {
	line := kwLineOf(node)
	for _, st := range lc.face.Statics {
		if st.Params["KeywordLine"] == line && param == "" {
			lc.out.LivingMetal = true
			return
		}
	}
	lc.refuseNode(node, "keyword %q without its expansion", "Living metal")
}
