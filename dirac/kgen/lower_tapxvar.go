package kgen

import "strings"

// tapXVarCost parses tapXType<X/Spec>: "tap X untapped <spec> you control"
// with X the count tapped (gorge cost.CostPart.Dyn "X", the tap election
// engine/rules/cumulative.go triggeredTapAsk poses; the answered count binds
// Count$xPaid). A printed trigger's own whole cost lowers it (Myr
// Battlesphere; payInstr's TapXVar part), and so does a printed activated
// ability's own cost with no other announced X (Aryel, Knight of Windgrace:
// gorge tapPermanentCostAsk's election announces X before the targets; the
// kernel's kop.CostTapX with N 0, cost_tapxann.go). Its SVar:X is the plain
// Count$xPaid; a spell's and an unless cost's X-form stay refused by name.
func (lc *lowerCtx) tapXVarCost(c *lCost, parts []string, t string) error {
	trig := lc.trigBodyCost && lc.ab != nil && lc.ab.Kind == akTriggered
	if !trig && !lc.actCost || c.TapXVar || c.TapN != 0 || c.TapHost || len(parts) < 2 {
		return refuse("tap cost %q outside a printed trigger's or activated ability's own cost", t)
	}
	if x, ok := lc.face.SVars["X"]; !ok || strings.TrimSpace(x) != "Count$xPaid" {
		return refuse("tapXType<X> cost with SVar:X other than Count$xPaid")
	}
	f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
	if err != nil {
		return err
	}
	// the untapped permanents the payer controls matching the spec (gorge
	// TapCostCandidates), as the literal tapXType<N/Spec> reads it
	for i := range f.Clauses {
		f.Clauses[i].Ctrl = whoYou
		f.Clauses[i].Flags |= fUntapped
	}
	c.TapXVar, c.TapF = true, f
	return nil
}
