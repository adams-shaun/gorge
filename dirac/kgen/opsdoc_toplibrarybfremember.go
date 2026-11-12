package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"c:LibTopRememberBF", "OpToLibrary C = kop.LibTopRememberBF",
			"The source itself, Origin$ Battlefield, to the top of its owner's library with RememberChanged$ True: the card moves only if it is still on the battlefield; one that reached the library joins the remembered set. Elixir of Immortality's first move, before its Shuffle Defined$ RememberedOwner."})
}
