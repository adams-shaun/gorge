// Champion (CR 702.72a) registered in kwHandlers. gorge kwChampion
// (cards/kw_champion.go) expands the printed line into one enters trigger
// whose body is the linked exile-until-leaves ChangeZone of another [type]
// you control (Duration$ UntilHostLeavesPlay supplies the return) with a
// sacrifice-unless-something-was-exiled sub-ability, and this lowering lowers
// that trigger. The printed keyword has no other rules meaning, so the
// handler only checks the expansion is present, the way lower_expandedkw4.go's
// heads do.
package kgen

func init() { kwHandlers["Champion"] = (*lowerCtx).lowerChampionKw }

func (lc *lowerCtx) lowerChampionKw(node, _ string) {
	k := kwLineOf(node)
	for _, t := range lc.face.Triggers {
		if t.Params["KeywordLine"] == k {
			return
		}
	}
	lc.refuseNode(node, "keyword %q without its expansion", "Champion")
}
