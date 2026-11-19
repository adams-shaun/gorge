package kgen

// Remember* writers the generic move lowering used to drop.
//
// ChangeZone Duration$ UntilHostLeavesPlay RememberChanged$ True (Hostage
// Taker): gorge effChangeZone remembers the exiled card in the resolution's
// set and on the source (eventRemember), and the chain's next link reads it
// (Effect RememberObjects$ Remembered with a MayPlay static over
// Card.IsRemembered). The O-Ring lowering emitted ExileUntilLeaves without
// the flag, so the kernel's set stayed empty and refuseCrossAbilityRemembered
// refused the reader as another ability's. OpExileUntilLeaves C MvRemember
// remembers the exiled object (dirac/interp_gen.go), and kremWrites sees the
// CS.

// exileUntilRememberCS is the ExileUntilLeaves CS for the move's flags:
// "Remember" when RememberChanged$ True set it, else none.
func exileUntilRememberCS(flags []string) string {
	for _, f := range flags {
		if f == "Remember" {
			return "Remember"
		}
	}
	return ""
}
