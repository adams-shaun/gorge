// Package lockstep plays a game on one set of card definitions, replays its
// recorded intents on another, and reports the first divergence.
//
// Forward plays on Forge's cards and replays on the swapped ones; Reverse is
// the other way round. The caller passes the Swap (gorgebe.Registry.Deck is
// one) and the bot factory, since engine packages import nothing outside
// engine/. The swap is outside replay.Replay's "same *cards.Card pointers"
// contract on purpose; TestSwapIsNeutral shows an identical swap is neutral.
package lockstep
