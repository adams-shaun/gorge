package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"stat:OptionalAttackCost", "StatOptionalAttackCost (no fields)",
		"The carrier's own exert offer (S:Mode$ OptionalAttackCost | ValidCard$ Card.Self | Cost$ Exert<1/CARDNAME>, CR 702.100a; gorge combat.go startExertAsks, exertOfferHolds): after the declaration's taps and Attacks triggers and after the band asks, each declared attacker carrying it (not under a lose-all-abilities effect, face up) asks its controller in declaration order (ctlExert, ChExert: the decline first, then the exert; canonical kind exert). A yes pays Exert<1/CARDNAME> (the one-shot no-untap mark, CostAtomExert's pfSkipUntap) and queues the card's own Exerted triggers; the declare-attackers priority round opens after the last answer. A gate (IsPresent$, Condition$, CheckSVar$: Combat Celebrant), another cost and a rider with a target stay refused. Khenra Scrapper, Glory-Bound Initiate. dirac/combat_exert.go; kgen/static_optattack.go."},
		opsDoc{"trig:Exerted", "TrigExerted (the card's own, no fields)",
			"Fired by the carrier's own exert yes (combat_exert.go fireExerted), after the declaration's triggers: the S:Mode$ OptionalAttackCost Trigger$ rider (gorge checkExertTriggers queues it off the Exert event) lowers as this self trigger over the SVar body. Only that static's rider lowers it: a printed Mode$ Exerted line (\"whenever you exert a creature\") reads other creatures and stays refused. Khenra Scrapper's +2/+0."})
}
