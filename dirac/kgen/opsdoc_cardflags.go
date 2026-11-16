package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// opsCardFlagUsage are the card-field rows of OPS.md whose census reads one
// field of the built CardDef: an opsdoc_<mechanic>.go registers its row in
// init() instead of growing opsUsage.
var opsCardFlagUsage []struct {
	label string
	on    func(d *kop.CardDef) bool
}

// addCardFlagUsage adds every supported card to the rows its CardDef sets.
func addCardFlagUsage(use map[string]map[string]bool, all []*lCard, bs []built) {
	for ci, c := range all {
		if len(c.Refusals) > 0 {
			continue
		}
		for _, f := range opsCardFlagUsage {
			if !f.on(&bs[ci].def) {
				continue
			}
			if use[f.label] == nil {
				use[f.label] = map[string]bool{}
			}
			use[f.label][c.Name] = true
		}
	}
}
