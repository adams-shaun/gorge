package kgen

func init() {
	opsCardFields = append(opsCardFields, opsDoc{"card:Dredge", "CardDef.Dredge", "Printed Dredge N (CR 702.52; gorge effects drawFor's dredge ask): whenever its owner would draw a card while it is in their graveyard and their library holds at least N cards, they may instead mill N and return it to their hand. The ask (a ChGenBranch decision, gorge's KModes) lists every such graveyard card in graveyard order, then the ordinary draw; each draw of a multi-card draw asks again. Posed by the turn draw and OpDraw; any other draw path with a candidate stalls (StallDredge). Kernel kw_dredge.go. Golgari Thug, Life from the Loam."})
}
