package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:DrawUpto", "OpExt ExtDrawUpto (Obj the drawer, B the most cards)",
			"Draw up to B cards (gorge effDraw's Upto$ True; Arcane Denial's \"may draw up to two cards\"): the drawer is asked how many, a Min 0 / Max min(B, library size) choice (one DkChoose ChDrawUpto pick per card over the top library cards in order, canonical kind card; Done ends the count) and draws that many. The drawer is the resolving controller or a player another reference names (RefTriggerPlayer in a delayed program registered with kop.DelayRemUpkeep). An empty library or a count of 0 asks nobody and draws nothing. The picks live in res.peek, no State byte. B is a constant 1..8 at lowering. Arcane Denial."},
		opsDoc{"delay:RemUpkeep", "OpDelayedProgram A = kop.DelayRemUpkeep (Obj a player ref, C ability)",
			"Register a delayed trigger at the next upkeep of any player whose fired program's trigger player (RefTriggerPlayer) is the player Obj names (gorge Phase$ Upkeep, ValidPlayer$ Player, RememberObjects$ RememberedController: the countered spell's controller, read as RefTargetController of the Counter's own target slot). The record holds the player in dprog.ext (dpExtRemUpkeep + the player) and no object, so the program's remembered set is empty. One-shot. Arcane Denial's \"its controller may draw up to two cards at the beginning of the next turn's upkeep\"."})
}
