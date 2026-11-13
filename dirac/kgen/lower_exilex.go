package kgen

import "strings"

// ExileFromGrave<X/Spec> (CR 601.2b, 107.3a; gorge cost.CostPart.Announced,
// kernel dirac/cost_exilex.go): "exile X cards from your graveyard" with the
// X the spell announces for its printed {X} (Skeletal Scrying's
// X B ExileFromGrave<X/Card>, SVar X Count$xPaid). Only a spell's own Cost$
// beside that {X}: gorge announces an X for the part even with no mana {X} and
// for an activated ability, where the kernel has no matching ask, and it
// announces a free X where the SVar names a fixed count (Araumi of the Dead
// Tide's "equal to the number of opponents", Forge's X = the SVar; gorge asks
// 0..graveyard instead, a gorge defect), so those stay refused.

// exileXCost reads ExileFromGrave<X/Spec> (spec not the source) into c: the
// announced X cards matching Spec are exiled (kop.CostExileGrave, N 0).
func (lc *lowerCtx) exileXCost(c *lCost, parts []string, t string) error {
	if len(parts) < 2 || len(parts) > 3 {
		return refuse("cost token %q", t)
	}
	if x, ok := lc.face.SVars["X"]; ok && strings.TrimSpace(x) != "Count$xPaid" {
		return refuse("exile cost %q with SVar X %q: gorge announces a free X where the script fixes the count", t, strings.TrimSpace(x))
	}
	if !lc.lifeXSpell {
		return refuse("exile cost %q outside a spell's own cost", t)
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
	c.ExileG, c.ExileGN, c.ExileGX = f, 0, true
	return nil
}
