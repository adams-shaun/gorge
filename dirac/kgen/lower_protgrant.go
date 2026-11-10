package kgen

import "strings"

// splitProtGrant takes the "Protection:<spec>" terms out of a Continuous
// static's AddKeyword$ list (Sword of Wealth and Power's "Protection:Instant
// & Protection:Sorcery"): the rest of the list and one filter holding every
// quality's clauses (protection from each is its own ability, CR 702.16g,
// so a source matching any of them is protected from). gorge reads the
// derived keyword through protectionQuality and sourceHasQuality, a filter
// over the source; the kernel reads kop.Static.ProtFrom the same way it
// reads a printed CardDef.ProtFrom. Only plain type and colour terms
// lower: a subtype term would need the matchup's subtype remap, and a
// controller or player term (Player.Opponent) is a different rule.
func (lc *lowerCtx) splitProtGrant(kw string) (string, *lFilter, error) {
	var rest []string
	var out *lFilter
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		head, param, ok := strings.Cut(k, ":")
		if !ok || head != "Protection" {
			rest = append(rest, k)
			continue
		}
		spec, _, _ := strings.Cut(param, ":")
		spec = strings.TrimSpace(spec)
		f, err := lc.filter(spec)
		if err == nil {
			err = plainQuality(f)
		}
		if err == nil {
			for _, c := range f.Clauses {
				if c.Subtype != "" || c.NotSubtype != "" {
					err = refuse("quality %s names a subtype", f)
				}
			}
		}
		if err != nil {
			return "", nil, refuse("keyword %q: %v", k, err)
		}
		if out == nil {
			out = f
		} else {
			out = &lFilter{Clauses: append(append([]lClause(nil), out.Clauses...), f.Clauses...)}
		}
	}
	return strings.Join(rest, " & "), out, nil
}
