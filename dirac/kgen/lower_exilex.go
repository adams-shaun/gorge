package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ExileFromGrave<X/Spec> (CR 601.2b, 107.3a; gorge cost.CostPart.Announced,
// kernel dirac/cost_exilex.go): "exile X cards from your graveyard" with the
// X the spell announces for its printed {X} (Skeletal Scrying's
// X B ExileFromGrave<X/Card>, SVar X Count$xPaid), or that a printed
// activated ability announces beside a printed {X} (Necropolis Fiend) or alone
// (Taigam, Sidisi's Hand's B T ExileFromGrave<X/Card>: gorge xAsk poses the X
// for the announced part, bounded by the matching graveyard cards). A spell's
// own Cost$ with no printed {X} (Gorex) and an escape cost stay refused
// (announcedXCheck, lower_escape.go). A script whose SVar X is not Count$xPaid
// fixes the count (gorge pay.FixLifeXCost drops the announcement): Araumi of
// the Dead Tide's "equal to the number of opponents" lowers as the live
// opponent count (kop.ExileGraveLiveCount, paid by act_forage.go); any other
// SVar stays refused.

// exileXCost reads ExileFromGrave<X/Spec> (spec not the source) into c: the
// announced X cards matching Spec are exiled (kop.CostExileGrave, N 0).
func (lc *lowerCtx) exileXCost(c *lCost, parts []string, t string) error {
	if len(parts) < 2 || len(parts) > 3 {
		return refuse("cost token %q", t)
	}
	fixed := false
	if x, ok := lc.face.SVars["X"]; ok && strings.TrimSpace(x) != "Count$xPaid" {
		// gorge (pay.FixLifeXCost) fixes the count to the SVar and asks
		// nothing; only the opponents' count of an activated ability's own
		// cost lowers, read live at payment (kop.ExileGraveLiveCount).
		if strings.TrimSpace(x) != "PlayerCountOpponents$Amount" || !lc.actCost {
			return refuse("exile cost %q with SVar X %q: the script fixes the count", t, strings.TrimSpace(x))
		}
		fixed = true
	}
	if !fixed && !lc.lifeXSpell && (!lc.actCost || lc.escCost) {
		return refuse("exile cost %q outside a spell's or activated ability's own cost", t)
	}
	if parts[1] == "CARDNAME" || c.ExileG != nil || c.ExileHandN != 0 || c.ExileGBf {
		return refuse("exile cost %q beside another exile part or of the source", t)
	}
	f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
	if err != nil {
		return err
	}
	for _, cl := range f.Clauses {
		if cl.CmpX != "" {
			return refuse("exile cost %q: a variable bound", t)
		}
	}
	if fixed {
		c.ExileG, c.ExileGN = f, kop.ExileGraveLiveCount
		return nil
	}
	c.ExileG, c.ExileGN, c.ExileGX = f, 0, true
	return nil
}
