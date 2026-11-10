package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:Vote", "OpExt ExtVote (F the ballot, C bit 0 exile the most-voted)",
			"Will of the council over a card ballot (CR 701.38a; gorge effCardVote): starting with the resolving controller, each player still in the game votes for one battlefield permanent matching F from the controller's view (one DkChoose ChVoteCard ask per voter, CanonKind vote_card). Every permanent with the most votes or tied for most is remembered in ballot order (seat 0's battlefield first, entry order); with C bit 0 (the VoteSubAbility$ that exiles the remembered set from the battlefield, the only one that lowers) they are then exiled together as one simultaneous departure, as gorge's ChangeZone moves a several-object set. An empty ballot asks nobody and remembers nothing. The votes live in res.arr, no State byte. Council's Judgment."})
}
