package kop

// ExtVote: will of the council over a card ballot (CR 701.38a; gorge
// effCardVote). Starting with the resolving controller, each player in turn
// order votes for one battlefield permanent matching F (read from the
// controller's view); every permanent with the most votes or tied for most
// is remembered, in the ballot order (seat 0's battlefield first, entry
// order). C bit 0: those permanents are then exiled together, one
// simultaneous departure (gorge's VoteSubAbility$ ChangeZone of Defined$
// Remembered to exile; Council's Judgment). No candidate: no player is asked
// and nothing is remembered. dirac/act_vote.go.
const ExtVote ExtOp = 311

func init() { extOpNames[ExtVote] = "Vote" }
