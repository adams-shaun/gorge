package kgen

// kwHandlers lowers a printed K: keyword that lowerKeywords' cases do not
// claim, keyed by its head: a keyword file registers its handler in init()
// instead of adding a case to lowerKeywords. A head with no handler refuses.
var kwHandlers = map[string]func(lc *lowerCtx, node, param string){}

// lowerKeywordElse is lowerKeywords' default arm.
func (lc *lowerCtx) lowerKeywordElse(node, head, param string) {
	if h := kwHandlers[head]; h != nil {
		h(lc, node, param)
		return
	}
	lc.refuseNode(node, "keyword %q", head)
}
