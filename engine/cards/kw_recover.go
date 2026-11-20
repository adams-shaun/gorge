// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// recoverExileSVar is the shared otherwise-exile branch: every Recover
// body's decline arm is the same cost-independent ChangeZone (Self, its
// graveyard -> exile), so one SVar per face serves every Recover line the
// face carries.
const recoverExileSVar = "__kwRecoverExile"

// kwRecover expands K:Recover:<cost> (CR 702.59) into the printed ability's
// two halves, both ordinary shapes the engine already carries:
//
//   - The trigger: "When a creature is put into your graveyard from the
//     battlefield" is a ChangesZone trigger (Origin$ Battlefield,
//     Destination$ Graveyard) whose ValidCard$ Creature.YouOwn
//     names the dead creature's OWNER ("your" graveyard is its owner's —
//     a card is put into its owner's graveyard, so owning the creature
//     suffices to own the destination). A carrier that is itself a
//     creature (Garza's Assassin, whose oracle says "another creature")
//     adds the +Other half — the corpus's own spelling for exactly this
//     shape (Kresh the Bloodbraided, "Whenever another creature dies") —
//     so its own corpse does not trigger its recover; for the six
//     noncreature carriers the clause is inert and the plain shape is
//     spelled. TriggerZones$ Graveyard keeps the trigger live only while
//     the Recover card is in its owner's graveyard (CR 702.59a: "Recover
//     functions only while this card is in a player's graveyard"), the
//     same zone the printed Bridge from Below triggers use.
//   - The resolution: the pay-or-else is the UnlessCost$ gate with
//     UnlessSwitched$ True — "paying CAUSES the effect" — so the payer
//     (UnlessPayer$ You, the Recover card's owner) is offered the printed
//     cost on resolution; paying runs the main body, a ChangeZone of Self
//     from its graveyard to its owner's hand. Declining (or being unable
//     to pay) runs the SubAbility$ branch instead: a ChangeZone of Self
//     from its graveyard to exile, gated UnlessResolveSubs$ WhenNotPaid.
//     Both branches find Self by its Origin$ Graveyard, so a card that
//     left the graveyard between the trigger and the resolution does
//     nothing (a new object, CR 400.7).
//
// param is the printed cost in Forge's spelling ("2 B", "R R", Garza's
// Assassin's `PayLife<X/half your life, rounded up>`, whose SVar:X the
// unless path's string resolver folds to half the payer's life, rounded
// up). No new parameter is introduced: every key the two bodies carry is
// already read (ChangeZone's census carries UnlessCost$/UnlessPayer$/
// UnlessSwitched$/UnlessResolveSubs$/SubAbility$, and the gate itself is
// engine/effects/unless.go's pre-dispatch implementation).
func kwRecover(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// param arrives trimmed from the boundary (keywords.go), so it is the
	// printed cost as-is.
	valid := "Creature.YouOwn"
	if f.IsCreature() {
		valid += "+Other"
	}
	f.setSVar(recoverExileSVar, "DB$ ChangeZone | Defined$ Self | Origin$ Graveyard | Destination$ Exile")
	f.addKeywordTrigger(head, k,
		"Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ "+valid+" | TriggerZones$ Graveyard | TriggerDescription$ Recover",
		"DB$ ChangeZone | Defined$ Self | Origin$ Graveyard | Destination$ Hand | UnlessCost$ "+param+
			" | UnlessPayer$ You | UnlessSwitched$ True | UnlessResolveSubs$ WhenNotPaid | SubAbility$ "+recoverExileSVar,
		has)
}

func init() { registerKeyword(kwRecover, "Recover") }
