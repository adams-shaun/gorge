package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// handManaLine lowers a Spirit Guide's mana ability (Simian / Elvish Spirit
// Guide: "Exile this card from your hand: Add {R}", AB$ Mana | Cost$
// ExileFromHand<1/CARDNAME> | Produced$ <one colour> | ActivationZone$ Hand;
// CR 605.1a, 605.2a: the ability functions from the hand) onto
// CardDef.HandMana, the colour it adds, and reports whether it did: the line
// is no CardDef ability. gorge's pay.ManaSourceIDs lists the hand after the
// battlefield, so the guide is a source of every manual payment window (its
// options after every battlefield source's, and the window's answer takes it
// only when no battlefield source is listed) and of every untapped-source gate
// (hasUntappedManaSource), while gorge's planner and potential census price no
// hand source and the oracle's canonical surface never activates a mana
// ability at priority (dirac/mana_handguide.go). The prefix searches
// fund with the guide too (filtSearch.handFund).
func (lc *lowerCtx) handManaLine(sa *cards.SA) bool {
	if sa.Kind != "AB" || sa.API != "Mana" || lc.out.HandMana != 0 {
		return false
	}
	var col kop.ColorMask
	for k, v := range sa.Params {
		v = strings.TrimSpace(v)
		switch k {
		case "Cost":
			if v != "ExileFromHand<1/CARDNAME>" {
				return false
			}
		case "ActivationZone":
			if v != "Hand" {
				return false
			}
		case "Produced":
			switch v {
			case "W":
				col = kop.ColW
			case "U":
				col = kop.ColU
			case "B":
				col = kop.ColB
			case "R":
				col = kop.ColR
			case "G":
				col = kop.ColG
			default:
				return false
			}
		case "SpellDescription":
		default:
			return false
		}
	}
	if col == 0 || sa.Params["ActivationZone"] == "" || sa.Params["Cost"] == "" {
		return false
	}
	lc.out.HandMana = uint8(col)
	return true
}

// handManaRefusal refuses a mana ability whose cost exiles its own source from
// the hand in a shape handManaLine does not lower (another production, another
// cost part beside the exile).
func handManaRefusal(kind int, cost string) error {
	if err := discardRandomManaRefusal(kind, cost); err != nil {
		return err
	}
	if kind != akMana || !strings.Contains(cost, "ExileFromHand<1/CARDNAME") {
		return nil
	}
	return refuse("exile cost %q on a hand-zone mana ability beside another part or production: only a Spirit Guide's own shape (one fixed colour) is a hand source of the kernel's mana windows", cost)
}
