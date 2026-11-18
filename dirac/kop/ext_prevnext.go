package kop

// The C bits of ExtPreventNext (dirac/repl_prevnext.go). PrevNextCombat and
// PrevNextAll name today's raw instruction flags 1 and 2; the rest name the
// rider bits of the later prevstate tickets.
const (
	// PrevNextCombat: the shield matches combat damage only (Decorated
	// Griffin's IsCombat$ True).
	PrevNextCombat = 1
	// PrevNextAll: no amount — an unbounded shield (col 255) that prevents
	// every event to its key until it ends (Shielded Passage; Awe Strike).
	PrevNextAll = 2
	// PrevNextFrom (instruction bit only; the record keeps a rider byte,
	// genState.prevX, whose pxFrom bit marks it): the shield is keyed on the
	// damage SOURCE (the remembered target creature, Awe Strike's
	// ValidSource$ Card.IsRemembered) instead of the recipient, and matches
	// only a source still on the battlefield at that object. It requires
	// PrevNextAll and is spent by the first event it prevents
	// (ExileOnMoved$ Battlefield ends it when the source leaves first).
	PrevNextFrom = 4
	// PrevNextGain (instruction bit only; the record keeps a rider byte,
	// genState.prevX, whose pxRidGain bit marks it): the shield's controller
	// gains life equal to the damage the event it prevents (Awe Strike's
	// ReplaceWith$ GainLifeInstead body).
	PrevNextGain = 8
	// Reserved for the later prevstate tickets; opPreventNext passes them
	// through and the installer stalls StallLowering on an unpaired use.
	PrevNextTokens    = 16
	PrevNextEndCombat = 32
	PrevNextPlayers   = 64
)
