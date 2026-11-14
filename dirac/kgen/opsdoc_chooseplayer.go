package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"op:ChoosePlayer", "OpChoosePlayer (Obj chooser, C ChPlRandom)",
			"The resolution's chosen player: a random pick among a player pool (gorge effChoosePlayer's Random$ True branch, CR 701.x's open choice made by the engine, not a decision). The pool is the lowered shape's: the chooser's living opponents that the source Aura is not attached to (Choices$ Player.Opponent+!EnchantedBy; gorge MatchesPlayerSpecWithSVars — the Opponent base reads against the chooser, !EnchantedBy is player.isEnchantedBy(source), and a source off the battlefield fails closed so excludes nobody), walked in AliveFrom order. C&ChPlRandom (the only lowered shape): the engine rng picks ONE of them with a single IntN(len(pool)) draw (gorge randomChoices); an empty pool draws nothing and records no choice. The pick is the resolution's chosen player, read as kop.RefResChosenPlayer (gorge Ctx.Chosen's player entry, Forge's chosen player field); it lasts the resolution (gorge's per-Ctx) and the Cleanup's ClearChosenPlayer$ is the no-op consumption kgen already models. gorge emits the choice's Note-level events only, which change no rules state. Maddening Hex's \"attach Maddening Hex to another one of your opponents chosen at random\"."})
}
