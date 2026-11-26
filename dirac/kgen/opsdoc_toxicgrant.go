package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:KwMarkToxic", "ExtKwMark C bit kop.KwMarkToxic, N in C's high byte (kop.KwMarkToxicShift)",
			"A Pump / PumpAll KW$ Toxic:N (CR 702.164, until end of turn; Aspirant's Ascent, Plague Nurse): the permanent's granted toxic count, kept in perm.spare's three toxic bits (no arena mask, no State field), cleared in the cleanup step. Toxic is cumulative (702.164c): each mark adds N, and combat damage poisons for the printed toxic plus every grant in force (State.toxicAt; gorge ToxicValue sums the derived Toxic:<N> entries). A mark past seven, or made while the permanent has lost its abilities, stalls StallToxicGrant. A Continuous static's AddKeyword$ Toxic:N is kop.Static.Toxic (Karumonix, Necrogen Communion, Prosthetic Injector) and an Animate's Keywords$ Toxic:N kop.Override.Toxic (Monument to Perfection), both layer-6 grants (one older than a loss of abilities grants nothing), summed beside the marks. dirac/kw_toxicgrant.go."},
	)
}
