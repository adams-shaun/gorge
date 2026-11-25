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
	// PrevNextTokens, PrevNextEndCombat and PrevNextPlayers are the riders of
	// Ria Ivor's source-keyed shield (repl_prevnext.go): the record's body
	// mints the instruction's token (the token pool index in B, mapped by
	// kgen's opPreventNext from the lInstr's CS key) as many as the damage it
	// prevents; the record ends with the combat; the shield matches a player
	// recipient only. All three are unpaired without PrevNextFrom, which the
	// installer stalls StallLowering on.
	PrevNextTokens    = 16
	PrevNextEndCombat = 32
	PrevNextPlayers   = 64
	// PrevNextChosen: the source-keyed shield's source is the kernel's chosen
	// set (the ChooseSource ask before it, dirac/act_choosesource.go) and its
	// recipient is the resolving controller as a player (ValidTarget$ You):
	// "the next time the chosen source would deal damage to you this turn,
	// prevent that damage" (Circle of Protection: Red). B is the
	// ChosenSrc* mask the source must still meet when the damage is dealt
	// (ValidSource$ Card.ChosenCardStrict+RedSource). It requires PrevNextAll
	// and PrevNextFrom and carries no rider.
	PrevNextChosen = 128

	// With PrevNextChosen, three upper bits that only the recipient-keyed
	// shields read otherwise name the chosen shield's recipient and lifetime
	// (dirac/repl_prevchosen.go):
	//
	// PrevNextChosenTo: the recipient is the instruction's Obj, a player or a
	// permanent at its current version (ValidTarget$ Card.IsRemembered,
	// Player.IsRemembered over a RememberObjects$ Targeted Effect: Martyr's
	// Cause; ValidTarget$ Card.IsImprinted over ImprintCards$ Targeted:
	// Samite Blessing).
	PrevNextChosenTo = PrevNextMultiObj
	// PrevNextChosenAny: any recipient (no ValidTarget$: Pilgrim of Justice).
	PrevNextChosenAny = PrevNextMultiPlayer
	// PrevNextChosenTurn: every event from the source this turn, never spent
	// (Prevent$ True with no body: Pay No Heed).
	PrevNextChosenTurn = PrevNextRadiance

	// The remaining instruction bits are the multi-recipient and rider kinds
	// (the record keeps a rider kind in genState.prevX, repl_prevx.go):
	//
	// PrevNextMultiObj: install a shield on every battlefield permanent
	// matching the instruction's filter F, amount B (Kitsune Palliator's
	// Defined$ Valid Creature, Samite Censer-Bearer's Creature.YouCtrl).
	PrevNextMultiObj = 1 << 8
	// PrevNextMultiPlayer: install a shield on every living player, amount B
	// (Kitsune Palliator's "each player" half).
	PrevNextMultiPlayer = 1 << 9
	// PrevNextRadiance: install a shield on the target in Obj and on each
	// other creature on the battlefield sharing a colour with it, amount B
	// (Wojek Apothecary's Radiance).
	PrevNextRadiance = 1 << 10
	// PrevNextDivided: the target slot in W was announced divided
	// (DividedAsYouChoose$); install one shield per still-legal chosen target
	// with its announced share (Angel of Salvation, Embolden, Pollen Remedy,
	// Remedy, Serra's Hymn).
	PrevNextDivided = 1 << 11
	// PrevNextRiderCounter: at each application put the prevented amount as
	// +1/+1 counters on the rider's target (Test of Faith, Temper).
	PrevNextRiderCounter = 1 << 12
	// PrevNextRiderDelayedCounter: accumulate the prevented amount and put it
	// as P0P1 counters on the rider's target at the beginning of the next end
	// step (Sacred Boon, Scars of the Veteran).
	PrevNextRiderDelayedCounter = 1 << 13
	// PrevNextRiderDamage: at each application deal the prevented amount as
	// damage from the shield's source to the rider's target (Acolyte's
	// Reward, Vengeful Archon).
	PrevNextRiderDamage = 1 << 14

	// ChosenSrc* are the type bits of a ChooseSource pool mask and of the
	// shield's damage-time re-check, beside the colour bits (kop.ColW ... ColG,
	// bits 0-4): a source qualifies when it has any named colour or type, and
	// a mask of 0 admits every source.
	ChosenSrcArtifact = 1 << 5
	ChosenSrcLand     = 1 << 6
)
