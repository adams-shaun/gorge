package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:PutCounterChoice", "OpExt ExtPutCounterChoice (Obj, B, C: up to four kinds)",
		"PutCounter whose CounterType$ lists counter kinds to choose from (gorge effPutCounter's counterKinds > 1 arm): the resolving controller picks one kind, one Min=Max=1 ChRemCtrKind decision over the listed kinds (mode = the counter kind, canonical as gorge's \"counter\" options), always asked, before the recipient is looked at; one counter of the pick is then put as an ordinary PutCounter (replacements, the CR 616.1 order ask, counter-added triggers). With Obj RefReplaced (an ETB$ True body, the lone instruction of an enters program) the ask is posed once the permanent is on the battlefield, as ExtEnterCtrPick's is, and the counter lands on it. The kinds are B's low byte, B's high byte, C's low byte and C's high byte, an 0xff byte ending the list. Boot Nipper, Owen Grady, Reluctant Role Model."})
}
