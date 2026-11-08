package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"c:LibTopGrave", "OpToLibrary C = kop.LibTopGrave",
			"A trigger object's Origin$ Graveyard move to the top of its owner's library: the card moves only if it is still in the graveyard; returned to a hand or exiled in response, it does not (gorge's Origin$ precondition on the object path; CR 400.7)."},
		opsDoc{"c:LibBottomGrave", "OpToLibrary C = kop.LibBottomGrave",
			"LibTopGrave with LibraryPosition$ -1: the card goes on the bottom of its owner's library only if it is still in the graveyard. Fell Horseman."})
}
