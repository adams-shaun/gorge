// The gap-4 keyword wave's four expanded heads (Soulshift, Reinforce, Embalm,
// Eternalize) registered in kwHandlers instead of a case in lowerKeywords'
// switch. Each head has no kernel-level meaning of its own: gorge's cards/
// kw_*.go expander turns the printed line into ordinary trigger / ability
// lines (tagged Keyword$ head, KeywordLine$ the line) and this lowering
// lowers those lines; the printed keyword gives the card no other rules
// meaning (CR 702.46a, 702.77a, 702.128a, 702.129a). So each handler lowers
// nothing itself: it only checks the face carries the expansion and refuses
// when it does not (a printed line whose expansion a printed-A:-line guard
// suppressed would otherwise lose its meaning silently), the way the Renown
// and Training arms of lowerKeywords check theirs.
package kgen

import "strings"

func init() {
	kwHandlers["Soulshift"] = (*lowerCtx).lowerSoulshiftKw
	kwHandlers["Reinforce"] = (*lowerCtx).lowerReinforceKw
	kwHandlers["Embalm"] = (*lowerCtx).lowerEmbalmKw
	kwHandlers["Eternalize"] = (*lowerCtx).lowerEternalizeKw
}

// kwLineOf recovers the full keyword line a lowerKeywordElse node carries:
// the node is "K:" + the line, the same string gorge's expandKeywords tags
// every expansion with as KeywordLine$.
func kwLineOf(node string) string { return strings.TrimPrefix(node, "K:") }

// lowerSoulshiftKw lowers K:Soulshift (CR 702.46a): gorge kwSoulshift
// (cards/kw_soulshift.go) expands it into one ChangesZone dies trigger
// (OptionalDecider$ You) whose body is DB$ ChangeZone | Origin$ Graveyard |
// Destination$ Hand of a Spirit card with mana value N or less, lowered from
// that line.
func (lc *lowerCtx) lowerSoulshiftKw(node, _ string) {
	k := kwLineOf(node)
	for _, t := range lc.face.Triggers {
		if t.Params["KeywordLine"] == k {
			return
		}
	}
	lc.refuseNode(node, "keyword %q without its expansion", "Soulshift")
}

// lowerReinforceKw lowers K:Reinforce (CR 702.77a): gorge kwReinforce
// (cards/kw_reinforce.go) expands it into one hand-zone AB$ PutCounter whose
// cost discards the card (the cycling shape), lowered from that line.
func (lc *lowerCtx) lowerReinforceKw(node, _ string) {
	if !lc.hasKeywordLine(kwLineOf(node)) {
		lc.refuseNode(node, "keyword %q without its expansion", "Reinforce")
	}
}

// lowerEmbalmKw lowers K:Embalm (CR 702.128a): gorge kwEmbalm
// (cards/kw_embalm.go) expands it into one graveyard sorcery-speed AB$
// CopyPermanent of Self whose cost is the parameter plus exiling the card
// itself, with the white Zombie (and Eternalize's black 4/4) copy
// exceptions, lowered from that line.
func (lc *lowerCtx) lowerEmbalmKw(node, _ string) {
	if !lc.hasKeywordLine(kwLineOf(node)) {
		lc.refuseNode(node, "keyword %q without its expansion", "Embalm")
	}
}

// lowerEternalizeKw lowers K:Eternalize (CR 702.129a): the same kwEmbalm
// expansion registered for both heads, the token additionally black and a
// 4/4, lowered from that line.
func (lc *lowerCtx) lowerEternalizeKw(node, _ string) {
	if !lc.hasKeywordLine(kwLineOf(node)) {
		lc.refuseNode(node, "keyword %q without its expansion", "Eternalize")
	}
}
