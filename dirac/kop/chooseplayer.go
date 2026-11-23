package kop

// OpChoosePlayer (gorge effChoosePlayer with Random$ True): the chooser Obj
// picks a player among the living opponents of theirs that the source Aura is
// not attached to (Choices$ Player.Opponent+!EnchantedBy; gorge
// MatchesPlayerSpecWithSVars: base Opponent read against the chooser,
// !EnchantedBy is player.isEnchantedBy(source), and a source off the
// battlefield fails closed so excludes nobody). The pick count is one
// (gorge choiceBounds' default); C&ChPlRandom says the engine rng picks it
// (gorge randomChoices: one rng.IntN(len(pool)) draw over the pool in
// AliveFrom order), never a decision, and an empty pool draws nothing and
// records no choice. The pick is the resolution's chosen player (res.chPl,
// read through RefResChosenPlayer); a choice the source's own Enchant:Player
// word would reject is re-checked at the attach (attachPlayerLegal, gorge
// effAttach's EnchantSelf walk). Maddening Hex's re-attach.
const OpChoosePlayer Op = 157

// ChPlRandom is OpChoosePlayer's C bit: the pick is the engine rng's (gorge
// Random$ True).
const ChPlRandom int16 = 1

// ChPlPlain is OpChoosePlayer's C bit 1: the pool is every living opponent of
// the chooser, the source Aura's bearer included (Choices$ Player.Opponent /
// Opponent with Random$ True: Knight Rampager, Raving Dead, Ruhan of the
// Fomori, ...). Clear, the bearer is left out (Choices$
// Player.Opponent+!EnchantedBy, Maddening Hex). Lowered programs keep the
// bit clear for Maddening Hex, so its recorded program is unchanged.
const ChPlPlain int16 = 2

func init() { opNames[OpChoosePlayer] = "ChoosePlayer" }
