package kgen

// patchIfPaidSkips: an AB$ Cost$ in a chapter body
// lowers to IfPaid with Skip -1 (lower_effect.go); like a printed
// trigger's (lower_trigger.go) it covers the rest of the chapter's
// program. gorge opens the same triggered-cost window for a chapter
// (rules/saga.go:63-70, stack.go:292-296).
func patchIfPaidSkips(ab *lAbility) {
	for i := range ab.Prog {
		if ab.Prog[i].Skip == -1 {
			ab.Prog[i].Skip = len(ab.Prog) - i - 1
		}
	}
}
