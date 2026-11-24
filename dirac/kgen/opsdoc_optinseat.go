package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:OptInSeat", "OpExt ExtOptInSeat (B the body's instructions a no skips)",
			"The head of an OpRepeatEachPlayer body under RepeatOptionalForEachPlayer$ True over Player.Opponent (gorge effRepeatEach's repeatEachElectionDecision, ResumeKind repeat_each_optional, a KChoose of yes / no whose Player is the subject): the repeated player, not the resolving controller, is asked yes / no (ChGenOptInSeat; canonical kinds yes_seat / no_seat, the named-seat pair appended to canon.ChooseKinds). A yes runs the body for that player; a no skips the body's other B instructions and the loop goes on to the next opponent, in seat order. Tempt with Bunnies and Tempt with Vengeance, whose body counts the accepted offers in a StoreSVar accumulator the chain reads after the loop and resets (a spell's write refuses unless its last write of the name is an unconditional write of the printed default). dirac/act_optinseat.go; dirac/oracle/tempt_optin_lockstep_test.go."})
}
