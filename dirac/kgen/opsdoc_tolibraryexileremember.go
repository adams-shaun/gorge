package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"c:LibBottomRememberExile", "OpToLibrary C = kop.LibBottomRememberExile",
			"A target, Origin$ Exile, to the bottom of its owner's library with RememberChanged$ True: the card moves only if it is still in exile; one that reached the library joins the remembered set (the ConditionDefined$ Remembered gate after it). Time Reaper's combat-damage trigger."})
}
