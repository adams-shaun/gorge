package kgen

// opsdoc_destaltask.go: the OPS.md row for OpAskDestAlt (emeria-shepherd-destzone).
func init() {
	opsEntries = append(opsEntries, opsDoc{
		"op:AskDestAlt",
		"OpAskDestAlt (A = the alternate's skip)",
		"Decision for the resolving controller: the primary or the alternate destination of the ChangeZone node that follows (gorge changezone_dest_alt, ChGenDestAlt, mode 0 = primary, 1 = alternate). Answer 0 falls into the next instruction (the primary arm); answer 1 skips the next A.N instructions (the primary arm and the jump over it) and runs the alternate arm. kgen emits it behind a SkipIf of the DestAltSVar$ condition, so it is posed only while the condition holds; DestAltSVar$ without MANDATORY from a non-library Origin$ (gorge asks in effChangeZone's prelude, before any move). Emeria Shepherd.",
	})
}
