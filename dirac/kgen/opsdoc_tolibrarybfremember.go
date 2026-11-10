package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"c:LibBottomRememberBF", "OpToLibrary C = kop.LibBottomRememberBF",
			"A delayed program's remembered object, Origin$ Battlefield, to the bottom of its owner's library with RememberChanged$ True: the card moves only if it is still on the battlefield; one that reached the library joins the remembered set. U.S.S. Farragut's end-step body."})
}
