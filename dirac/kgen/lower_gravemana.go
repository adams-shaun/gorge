package kgen

import (
	"fmt"
	"io"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// graveManaLine records a graveyard mana source's colours on CardDef.GraveMana
// (Jack-o'-Lantern: AB$ Mana | Cost$ 1 ExileFromGrave<1/CARDNAME> |
// ActivationZone$ Graveyard | Produced$ Any; CR 605.1a, 605.2a) and reports
// whether the line has that exact shape: a lone generic {1} fee beside the
// CARDNAME exile, one mana of any colour, and no other parameter. The byte is
// the whole model (dirac/mana_gravemana.go): the kernel's payment window and
// the prefix search's funding pass read it, pay the {1} fee from the floating
// pool and exile the card, so the line lowers no ability -- a mana ability
// here would be one the kernel's mana walk cannot honour (build.go refuses
// it). The card is two-seat: the window and the funding pass play in a
// two-seat game, and a pod of more seats refuses the table (StallTwoSeat).
func (lc *lowerCtx) graveManaLine(sa *cards.SA) bool {
	if sa.Kind != "AB" || sa.API != "Mana" || lc.out.GraveMana != 0 || !graveManaParams(sa) {
		return false
	}
	lc.out.GraveMana = uint8(kop.ColAny)
	lc.twoSeat("the grave mana source's activation window and funding pass play in a two-seat game (dirac mana_gravemana.go)")
	return true
}

// graveManaParams: sa's parameters are exactly Jack-o'-Lantern's mana line's.
func graveManaParams(sa *cards.SA) bool {
	for k, v := range sa.Params {
		v = strings.TrimSpace(v)
		switch k {
		case "Cost":
			if v != "1 ExileFromGrave<1/CARDNAME>" {
				return false
			}
		case "ActivationZone":
			if v != "Graveyard" {
				return false
			}
		case "Produced":
			if v != "Any" {
				return false
			}
		case "SpellDescription":
		default:
			return false
		}
	}
	return sa.Params["Cost"] != "" && sa.Params["ActivationZone"] != "" && sa.Params["Produced"] != ""
}

// dumpZoneMana writes c's hand-zone and graveyard-zone mana bytes to dumpLIR's line.
func dumpZoneMana(w io.Writer, c *lCard) {
	if c.HandMana != 0 {
		fmt.Fprintf(w, " handmana=%#x", c.HandMana) // lower_handmana.go
	}
	if c.GraveMana != 0 {
		fmt.Fprintf(w, " gravemana=%#x", c.GraveMana)
	}
}
