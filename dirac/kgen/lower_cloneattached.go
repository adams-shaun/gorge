package kgen

// cloneAttached lowers Assimilation Aegis's Attached-trigger body:
//
//	DB$ Clone | Choices$ Creature.ExiledWithSource | CloneTarget$ TriggeredTargetLKICopy
//	          | Duration$ UntilUnattached | ChoiceZone$ Exile
//
// exactly, inside a Mode$ Attached trigger; every other Clone is refused. The
// kernel's ExtAegisClone reads the exile-until-leaves link of the trigger's
// source, so the source must be the card that exiled the creature (the same
// card's ETB trigger; checked at lockstep, not here: a card with the
// Attached body and no exiling ETB has an empty pool and does nothing).
func (lc *lowerCtx) cloneAttached(ps *params) error {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "Attached" || !lc.trigBody {
		return lc.cloneCopy(ps) // lower_clonecopy.go
	}
	want := [][2]string{
		{"Choices", "Creature.ExiledWithSource"},
		{"CloneTarget", "TriggeredTargetLKICopy"},
		{"Duration", "UntilUnattached"},
		{"ChoiceZone", "Exile"},
	}
	for _, w := range want {
		if v := ps.str(w[0]); v != w[1] {
			return refuse("Clone %s$ %q", w[0], v)
		}
		ps.consume(w[0])
	}
	lc.emit(lInstr{Op: "AegisClone"})
	return nil
}
