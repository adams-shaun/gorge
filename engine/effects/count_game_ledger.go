package effects

// countGameLedger reads the controller's game-long ledger counts, each derived
// through the Host from the event log so a replay derives the same number:
//
//   - YourTurns: how many of the game's turns have begun with the controller as
//     the active player, current turn included (a "first, second, or third turn
//     of the game" gate).
//   - YourExtraTurns: how many extra turns the controller has begun this game
//     (a "haven't begun an extra turn this game" gate).
//   - YouCastThisGame: how many spells the controller has cast this game (CR
//     601.2i: a spell is cast once it is paid for; copies and abilities are
//     never cast), counting the resolving spell itself once its cast happened.
func countGameLedger(h Host, c *Ctx, code evalCountBodyPaidCode) int32 {
	switch code {
	case evalCountBodyPaidYourExtraTurns:
		return h.ExtraTurnsBegun(c.Controller)
	case evalCountBodyPaidYouCastThisGame:
		return h.SpellsCastThisGame(c.Controller)
	}
	return h.TurnsTaken(c.Controller)
}
