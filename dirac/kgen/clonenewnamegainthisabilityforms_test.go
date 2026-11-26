package kgen

import "testing"

// DB$ Clone with the CR 707.9b exception GainThisAbility$: the
// copy keeps the printed ability this Clone is the body of (AddSVars$ only
// carries the SVars that ability reads).
func TestCloneExceptionsForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Artisan of Forms", form: "GainThisAbility$ + AddSVars$, a triggered ability (Optional$)", lir: "CloneCopy"},
		{card: "Cryptoplasm", form: "GainThisAbility$ + AddSVars$, an upkeep trigger", lir: "CloneCopy"},
		{card: "Protean Thaumaturge", form: "GainThisAbility$ + AddSVars$, an ETB-style trigger", lir: "CloneCopy"},
		{card: "Irma, Part-Time Mutant", form: "NewName$ still refuses (the kernel's copy exceptions carry no name)", refused: "Clone NewName$ on the battlefield"},
		{card: "Aurora Shifter", form: "AddTriggers$ beside GainThisAbility$ still refuses", refused: "Clone AddTriggers$ beside GainThisAbility$"},
		{card: "Absorbing Man", form: "NewName$ still refuses", refused: "Clone NewName$ on the battlefield"},
		{card: "Likeness Looter", form: "TgtZone$ Graveyard without UntilEndOfTurn still refuses", refused: "Clone TgtZone$ Graveyard without Duration$ UntilEndOfTurn"},
	})
}
