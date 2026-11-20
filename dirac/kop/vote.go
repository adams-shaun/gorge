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

// The fixed-list Vote's C bits (dirac/act_vote.go).
const (
	// VoteStore is StoreVoteNum$ True: after the last voter, every Choices$
	// option branch runs in order with the tally register set to that
	// option's count (gorge resolveVoteOutcomes' StoreVoteNum branch).
	VoteStore = 1 << 6
	// VoteBranchStore flags an OpBranch C as a StoreVoteNum option branch:
	// in VoteStore mode the kernel runs every flagged branch in order and
	// sets the current tally to the branch index; otherwise the branch
	// compares its low three bits with the selected winner as usual.
	VoteBranchStore = 1 << 7
)

func init() { extOpNames[ExtVote] = "Vote" }
