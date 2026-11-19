// Package lab is a small wrapper for playing games and setting up experiments
// on dirac, with no other engine in the loop.
//
// A Config names the card set and one deck per seat (two to dirac.MaxSeats);
// NewGame (or Open then Setup.Game) deals a game. A Game exposes the pending
// decision with readable actions, Apply, Fork, Done and a position Hash. A
// Policy answers decisions: Random, HBot and Search are built in. Play runs a
// game to its end and Batch runs many in parallel and summarises them.
//
//	g, _ := lab.NewGame(lab.Config{Set: "fdn", Decks: []lab.Deck{a, b}, Seed: 1})
//	r := lab.Play(g, []lab.Policy{lab.HBot(), lab.Random(1)}, 20000)
//
// A game that dirac cannot continue (a capacity or a rule gap) is Stalled and
// one that ran out of steps is Truncated. Neither is a draw, and neither is
// counted as a win for anyone. The package reads no hidden zone of any game: a
// policy sees the State and Decision dirac gives it, as dplay's seats do.
//
// It imports dirac, dirac/cardset, dirac/kop, dirac/search/... and
// engine/deck (to read deck files). Games are not safe for concurrent use; a
// Batch gives every worker its own.
package lab
