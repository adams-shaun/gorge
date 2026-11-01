package kgen

// alterAttribute lowers api:AlterAttribute Attributes$ Saddled (the Saddle
// expansion's body; gorge effAlterAttribute "Saddled": the designation,
// CR 702.171b) as OpSaddle. The op takes any object ref, so the designation
// may ride the source (Defined$ Self), a target slot (Defined$ absent
// beside ValidTgts$, or Defined$ Targeted) or the trigger object (Defined$
// TriggeredCard), as gorge's Defined() loop reads each. Every other
// attribute, Activate$ and every other Defined$ refuses.
func (lc *lowerCtx) alterAttribute(ps *params) error {
	if ps.str("Attributes") == "Prepared" {
		return lc.alterPrepared(ps, lRef{Kind: rSelf})
	}
	a := ps.str("Attributes")
	if a == "Plotted" {
		return lc.alterPlotted(ps)
	}
	if a == "Suspected" {
		return lc.alterSuspected(ps) // lower_suspect.go
	}
	if a != "Saddled" && a != "Solved" {
		return refuse("AlterAttribute Attributes$ %q", a)
	}
	if d := ps.str("Defined"); d != "Self" {
		_, hasTgt := ps.get("ValidTgts")
		ok := a == "Saddled" && (d == "Targeted" || d == "TriggeredCard" || d == "" && hasTgt)
		if !ok {
			return refuse("AlterAttribute Defined$ %q", d)
		}
	}
	if _, ok := ps.m["Activate"]; ok {
		return refuse("AlterAttribute Activate$")
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if a == "Solved" {
		// a Case's solved designation (CR 719.3b; gorge effAlterAttribute
		// Solved): OpSaddle with C = 1, the same per-permanent bit, never
		// cleared as a turn begins (kernel kw_crewsaddle.go). A precedent,
		// not a pattern: it is safe only because a Case is never saddled.
		// A new family takes an OpExt sub-op (kop/ext.go), not another
		// op's spare field
		lc.emit(lInstr{Op: "Saddle", Obj: obj, C: 1})
		return nil
	}
	lc.emit(lInstr{Op: "Saddle", Obj: obj})
	return nil
}

// alterPlotted lowers AlterAttribute Attributes$ Plotted Defined$
// Remembered (Kellan Joins Up) as OpExt ExtPlotGrant: gorge
// effAlterAttribute gates Plotted on exile and grants the designation to
// each Defined$ object there (CR 702.170b-d). Another Defined$, Activate$
// (a removal) or a target refuses.
func (lc *lowerCtx) alterPlotted(ps *params) error {
	if d := ps.str("Defined"); d != "Remembered" {
		return refuse("AlterAttribute Plotted Defined$ %q", d)
	}
	if _, ok := ps.m["Activate"]; ok {
		return refuse("AlterAttribute Plotted Activate$")
	}
	if _, ok := ps.m["ValidTgts"]; ok {
		return refuse("AlterAttribute Plotted ValidTgts$")
	}
	obj, err := lc.defined("Remembered")
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "PlotGrant", Obj: obj, Note: "the remembered exiled cards become plotted"})
	return nil
}
