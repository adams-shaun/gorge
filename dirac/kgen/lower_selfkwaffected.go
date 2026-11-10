package kgen

// selfkwaffected: a keyword-writing Continuous static whose Affected filter
// tests a keyword through with<Keyword>/without<Keyword> (a static granting
// indestructible to its source while it has defender). The kernel reads
// that test through the permanent's keywords, which evaluate the same static
// again, without end. The layer-6 walk orders only the flying gate
// (dirac/static_layer6.go filterReadsKw, gorge abilityDependencyOrder), so
// any other keyword gate beside a keyword write is refused.

// affectedKwCheck refuses st when it writes keywords and its Affected
// filter tests one through a keyword comparison.
func affectedKwCheck(st *lStatic) error {
	if st.F == nil || !staticWritesKw(st) {
		return nil
	}
	for _, c := range st.F.Clauses {
		if c.CmpStat == cmpHasKw || c.CmpStat == cmpLacksKw {
			return refuse("keyword static whose Affected$ tests a keyword (layer-6 self-dependency)")
		}
	}
	return nil
}

// staticWritesKw: st grants, removes or loses keywords.
func staticWritesKw(st *lStatic) bool {
	if len(st.Kw) > 0 {
		return true
	}
	o := st.Override
	return o != nil && (o.LoseAbilities || len(o.AddKw) > 0 || len(o.RemoveKw) > 0)
}
