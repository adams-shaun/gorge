package kgen

import "strings"

// filterPropAlias rewrites a filter property spelling gorge reads as the
// same predicate as one prop already lowers:
//
//   - with<Keyword>/!with<Keyword>/without<Keyword> naming a two-word
//     keyword as Forge prints it ("withFirst Strike"): gorge
//     keywordPredicateFor compacts the spaces out of the with/without forms
//     before the registry lookup, so it is withFirstStrike.
func filterPropAlias(p string) (string, bool) {
	for _, pre := range [...]string{"!with", "without", "with"} {
		if kw, ok := strings.CutPrefix(p, pre); ok {
			switch kw {
			case "First Strike", "Double Strike":
				return pre + strings.ReplaceAll(kw, " ", ""), true
			}
			return "", false
		}
	}
	return "", false
}
