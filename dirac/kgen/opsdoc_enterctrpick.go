package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:EnterCtrPick", "OpExt ExtEnterCtrPick (B, C: the three kinds)",
		"An as-enters choice of two different counter kinds from three, the only instruction of an enters program (CR 614.1c, 614.12; gorge's ETB PutCounter with CounterType$ A,B,C and ChooseDifferent$ 2: Grimdancer). gorge runs the body once the permanent is on the battlefield, so the kernel queues the ask then: two sequential ChRemCtrKind decisions (mode = the counter kind, canonical as gorge's \"counter_kinds\" options), the first over all three kinds, the second over the two left; the permanent then has one counter of each picked kind (placed as an ordinary put, so counter replacements and counter-added triggers see them). The kinds are B (first), C's low byte (second) and C's high byte (third). Grimdancer."})
}
