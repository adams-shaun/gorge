package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:EnterDevour", "OpExt ExtEnterDevour (B: counters per sacrifice, F: the sacrificed permanents' filter)",
		"An as-enters Devour (CR 702.82a; gorge's engine/cards/kw_devour.go expansion, one Moved ETB replacement: an optional sacrifice of any number of the controller's permanents matching F). The lone instruction of an enters program; gorge's ReplacementResult$ Updated body runs once the permanent has entered, so the kernel queues the ask then: one ChSacrifice decision per pick over the unpicked matches in battlefield order, Done always offered (gorge's Min 0, Max the eligible count KChoose). The picks stay on the battlefield until the answer commits and leave as one batch in pick order; the permanent then gets B +1/+1 counters for each one (placeEntryCounters). Famished Worldsire, Feasting Hobbit. dirac/kw_devour.go."})
}
