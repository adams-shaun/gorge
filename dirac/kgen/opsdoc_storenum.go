package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:StoreNum", "OpExt ExtStoreNum (B the value, C name | delete flag | printed default)", "StoreSVar's number write onto the resolving source (gorge effStoreSVar, events.StoreSVar -> Object.RuntimeSVars, last write wins): B is the amount (Type$ Number, Calculate or CountSVar through the Expr tree, `X/Plus.1` = ExSVarNum X + 1), C the SVar's Table.SVarNames index, StoreNumDel and the printed default, so a write of the default deletes the record. The record lives in the object-set store (kind osNum) keyed by the object, whatever zone it is in: only a departure from the battlefield clears it (CR 400.7; gorge's wasBattlefield reset, so a write after the source left persists on the departed object, as gorge's does), and a full store or an int16 overflow stalls StallObjSets. kgen lowers an activated ability's or a trigger's write only; a spell's, a mana or latent ability's, Type$ Triggered/Targeted and a table's 16th name refuse by name. dirac/act_storenum.go. A synthetic probe in the lockstep (dirac/oracle/numstore_lockstep_test.go); no pooled card yet."},
		opsDoc{"ex:SVarNum", "ExSVarNum (N name index, Sub[0] the printed body)", "The evaluating source's stored number for the name (ExtStoreNum), else Sub[0], the printed SVar body (gorge runtimeSVar is checked before the printed table, effects/count.go). Every reader of a stored name lowers to it through lc.value; the modifiers (`/Plus.1`) ride the Expr's Add. dirac/act_storenum.go."},
	)
}
