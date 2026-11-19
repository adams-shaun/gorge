// Package shadow stages a dirac State from what a seat can see.
//
// A bot attached to an external engine never holds that engine's omniscient
// state: it holds its seat's observation (engine/view.View built with seat
// visibility), and to search it needs a world, a guess at every hidden zone.
// This package turns an observation plus a caller-supplied assignment of the
// hidden zones into a dirac.World, and that into a dirac.State
// (dirac.NewFromWorld). It reads nothing else: no engine/state, no
// clairvoyant option, no hidden zone of the game it observes.
//
// The hidden assignment (Hidden) is the caller's: a determinization deals it
// from the seat's own deck list and the counts the view shows, and a
// verifier supplies the true one. A position outside the minimal staging
// scope is a dirac.ErrUnstageable, never a partial or guessed State.
package shadow
