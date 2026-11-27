package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// maxManaCostWord is gorge's largest registered ManaCost<N> word
// (engine/effects/filter_manacost.go).
const maxManaCostWord = 15

// ManaCost<N>: the candidate prints a mana cost with mana value N (gorge
// ManaCost<N>; kernel FwManaCostEQ over N in Arg). Unlike cmcEQ<N> it never
// matches a printed "no cost". Urza's Saga's Chapter III search.
func init() {
	for n := 0; n <= maxManaCostWord; n++ {
		n := n
		fwArms["ManaCost"+strconv.Itoa(n)] = func(c *lClause, _ string) error {
			return c.fwLit(lSpillLit{W: kop.FwManaCostEQ, N: n})
		}
	}
}

// isManaCostWord reports whether p is a registered ManaCost<N> word.
func isManaCostWord(p string) bool {
	d, ok := strings.CutPrefix(p, "ManaCost")
	if !ok {
		return false
	}
	_, has := fwArms[p]
	return has && d != ""
}
