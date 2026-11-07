package rules

// refreshEntryTriggerTables brings the layer-3 rename and layer-4 derived-type
// tables up to the board an entry move has just folded, before that entry's
// triggers are matched. The Updated-replacement entry paths fold the move with
// a raw events.Emit, which bypasses Engine.emit's own refresh tail, so a
// ValidCard$ on the entering permanent would otherwise read the table of the
// board before it entered: a type its own static grants from its as-enters
// choice (Adaptive Automaton's AddType$ ChosenType, CR 614.12) was missing, and
// an "another Angel enters" trigger did not fire (CR 603.6a).
func (e *Engine) refreshEntryTriggerTables() {
	if e.setNameInPool {
		e.refreshRenames()
	}
	if e.layer4InPool {
		e.refreshDerivedTypes()
	}
}
