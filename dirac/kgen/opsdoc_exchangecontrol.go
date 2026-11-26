package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:ExchangeControl", "OpExt ExtExchangeControl (C target-slot bit set)",
		"DB$ ExchangeControl (CR 701.12; gorge effects/exchange_control.go effExchangeControl). The battlefield objects the slots in C name that are still legal at resolution, in slot then pick order, are the pair: when exactly two remain and their controllers differ, each goes to the other's controller with no duration (one permanent control record each, CR 613.1b). Anything else does nothing. Either one slot of exactly two targets (optionally TargetsWithDifferentControllers$) or an earlier link's single target (Defined$ ParentTarget) beside this link's own. Obj RefSelf (Defined$ Self beside one target slot: Avarice Totem, Phyrexian Infiltrator) makes the source the pair's first member, the target its second; a source no longer on the battlefield as the same object, or one that targeted itself, is no pair."})
}
