package kgen

import (
	"strconv"
	"strings"
)

// ExiledMoveToGrave<N/Spec[/description]> (gorge cost.parse's MoveToGrave
// part, Forge CostExiledMoveToGrave; CR 118.3): N exiled cards matching Spec
// go to their owners' graveyards as the cost is paid. Only a printed
// trigger's own Cost$ lowers it (the Eldrazi processors' "you may put a card
// an opponent owns from exile into that player's graveyard. If you do, ...":
// gorge's triggered-cost window, kernel cost_exiledmovegrave.go), and only a
// source-free Spec. Refused by name:
//   - an activated ability's or a spell's own cost (Cryptic Cruiser,
//     Processor Assault): gorge's cast-time moveGraveAsk pick has no kernel
//     cost-pick stage yet;
//   - Card.ExiledWithSource (Jester's Scepter, Foreboding Steamboat, Shelob):
//     the source-relative exile link is not read by this pick.
func (lc *lowerCtx) exiledMoveGraveCost(c *lCost, parts []string, t string) error {
	if !lc.trigBodyCost {
		return refuse("cost token %q outside a trigger's own cost", t)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || len(parts) < 2 || len(parts) > 3 || c.MoveGraveN != 0 {
		return refuse("cost token %q", t)
	}
	if strings.Contains(parts[1], "ExiledWith") {
		return refuse("cost token %q: an ExiledWithSource spec", t)
	}
	if n < 1 || n > 15 {
		return refuse("cost token %q", t)
	}
	f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
	if err != nil {
		return err
	}
	c.MoveGraveN, c.MoveGraveF = n, f
	return nil
}
