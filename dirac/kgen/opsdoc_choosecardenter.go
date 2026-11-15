package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:ChooseCardEnter", "OpExt ExtChooseCardEnter (F)",
		"An as-enters choice of another creature its controller controls, the only instruction of an enters program (CR 614.12; gorge's ETB ChooseCard, Defined$ You, Choices$ Creature.YouCtrl+Other, Mandatory$ True). gorge runs the body once the permanent is on the battlefield, so the kernel queues the ask then (never with the spell on the stack): a ChGenChooseCard decision over the battlefield objects matching F other than the entrant, source and mode the object, posed even with one candidate and not at all with none. The answer is kept in the permanent's chosen byte as the Obj (a permanent that left holds nothing, CR 400.7). A resolution of the card's own ability or trigger starts its chosen set with it, from the entry's last known information once a Sac<1/CARDNAME> cost moved the source (CR 608.2h; gorge TriggerContext.SourceChosen); the ability's Defined$ Valid Card.ChosenCardStrict lowers to that chosen object (RefChosen; ChosenCardStrict is no filter word). Dauntless Bodyguard."})
}
