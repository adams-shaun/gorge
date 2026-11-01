// Package mbtest is the ManaBrew adapter's test support: a deterministic mock ManaBrew client that answers every
// prompt type server/manabrew's Translator can build, a seat.Seat wrapper
// (TranslatingSeat) that drives decision -> prompt -> mock response -> intent
// in-process, and the census/replay-equivalence tests that exercise both
// against real repo-deck games.
//
// It is test support, not the engine or the wire transport: it imports
// server/manabrew, server/protocol/manabrew, seat and testkit/bench. HTTP lives
// in server/host/manabrewhttp.
package mbtest
