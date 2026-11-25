package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

func init() { exprKinds[vAltCost] = kop.ExAltCost }

// altCostRead admits Count$AltCost.<paid>.<unpaid> (the Mastery cycle):
// the kernel reads the resolving spell entry's trigPlAltCost mark, which
// only a cast for the card's own S:Mode$ AlternativeCost static through
// castAltTgt sets (dirac/value_altcost.go). So the read is lowered only in a
// spell ability of a face with exactly one such static whose cost is not
// free (a free alternative rides the may-play free-cast mode, which carries
// no mark), and with none of the keyword alternatives gorge also stamps
// FlagAltCostPaid for (rules altcast_modes.go: Madness, Prowl, Freerunning,
// Awaken; rules emerged_cast.go: Emerge).
func (lc *lowerCtx) altCostRead() error {
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return refuse("Count$AltCost outside a spell ability")
	}
	for _, kw := range lc.face.Keywords {
		switch strings.ToLower(cards.KeywordHead(kw)) {
		case "madness", "prowl", "freerunning", "awaken", "emerge":
			return refuse("Count$AltCost on a face with a keyword alternative cost (%s)", cards.KeywordHead(kw))
		}
	}
	n := 0
	for i := range lc.face.Statics {
		st := &lc.face.Statics[i]
		if st.Mode != "AlternativeCost" {
			continue
		}
		n++
		if st.Params["ValidSA"] != "Spell.Self" {
			return refuse("Count$AltCost beside an AlternativeCost static other than Spell.Self")
		}
		if c := strings.TrimSpace(st.Params["Cost"]); c == "" || c == "0" || c == "Reveal<1/Hand>" {
			return refuse("Count$AltCost with a free alternative cost")
		}
	}
	if n != 1 {
		return refuse("Count$AltCost needs exactly one own AlternativeCost static (%d)", n)
	}
	return nil
}
