package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Will of the council: either over a card ballot (CR 701.38a; gorge
// effCardVote, engine/effects/vote_effect.go) or over a fixed list of SVar
// choices (gorge effVote's Choices$ shape). The card ballot is every
// battlefield permanent matching the op's filter from the resolving
// controller's view. Starting with that controller, each player still in the
// game votes (gorge askFixedVote/askCardVote: one KChoose per voter, Min =
// Max = 1). Every permanent with the most votes or tied for most is
// remembered, in ballot order (seat 0's battlefield first, entry order); with
// C bit 0 they are then exiled together (Council's Judgment's sub-ability,
// which gorge moves as one simultaneous departure batch).
//
// The fixed-list ballot (C bit 1) carries 2..4 choice indexes in C bits 2..4:
// each voter picks one index in Choices$ order, and the winning branch (or
// the VoteTiedAbility$ branch, C bit 5) is selected exactly as gorge's
// voteWinner: the first maximum wins, a tie is when two or more choices share
// the maximum.
//
// The votes cast so far live in res.arr[:res.left] (one Obj per voter: the
// voted permanent, or the choice index + 1), so the tally needs no scratch of
// its own: it is counted once every vote is in. The card ballot is recomputed
// for each ask; nothing changes between the asks.

// raVote is the vote ask. It sits past the ask families that offset from
// raGenManifestDread (+1..+3, +40..+45, +50, +60, +61, +70..+77).
const raVote = raGenManifestDread + 78

// The ExtVote operand's mode bits (the op's C).
const (
	voteCExile = 1 << 0        // the card ballot's exile of the most-voted
	voteCFixed = 1 << 1        // the fixed-list shape, not the card ballot
	voteCTie   = 1 << 5        // a VoteTiedAbility$ branch exists
	voteCStore = kop.VoteStore // StoreVoteNum$: every option branch runs with its tally
	voteCShift = 2             // fixed-list choice count
	voteCMask  = 7
)

// VoteStats counts the fixed-list vote rounds of a run; oracle tests only.
type VoteStats struct {
	Rounds    int // fixed-list votes resolved
	Ties      int // of those, a shared maximum
	NonTies   int // of those, a unique maximum
	TieBranch int // of the ties, rounds whose VoteTiedAbility$ branch ran
	// StoreRounds is the fixed-list votes resolved in StoreVoteNum$ mode,
	// where every Choices$ option branch runs; StoreMulti is how many of
	// those had two or more options with a nonzero tally.
	StoreRounds int
	StoreMulti  int
}

// DebugVoteStats is the test hook; nil in normal play.
var DebugVoteStats *VoteStats

// DebugVoteNumWinnerOnly (TestVoteStoreNumMutation): a StoreVoteNum$ vote
// resolves only the winning option instead of every Choices$ option.
var DebugVoteNumWinnerOnly bool

// voteStoreAll arms the scratch for a StoreVoteNum$ fixed-list vote: every
// option branch runs in Choices$ order, each with the current tally set to
// its own count (gorge resolveVoteOutcomes' StoreVoteNum branch, zero-tally
// options included).
func (r *armRes) voteStoreAll(counts [voteCMask + 1]uint8, n int) {
	r.voteCounts = counts
	r.voteAllN = uint8(n - 1)
	r.voteAll = true
	r.voteCur = 0
}

// DebugVoteTieTakesFirst (TestVoteChoicesTieMutation): a tie resolves the
// first tied option instead of the VoteTiedAbility$ branch.
var DebugVoteTieTakesFirst bool

// voteChoiceCount is the fixed-list ballot's option count (C bits 2..4).
func voteChoiceCount(in *kop.Instr) int { return int(in.C>>voteCShift) & voteCMask }

// voteBallot is the card ballot's candidates: the battlefield slots matching
// in.F for the resolving controller.
func (s *State) voteBallot(k *prep, in *kop.Instr) slotMask {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	var m slotMask
	for occ := s.occ; occ.any(); occ = occ.rest() {
		if sl := occ.first(); s.matchPerm(k, in.F, sl, e.ctrl, src) {
			m = m.with(sl)
		}
	}
	return m
}

// voteNext poses the vote of the next player in turn order from the
// controller who has not voted and is still in the game; false when every
// voter is done. res.chosen is the turn-order offset of the next voter, so a
// departed seat is skipped without shifting the seats after it.
func (s *State) voteNext() bool {
	n := s.nSeats()
	for i := uint8(s.res.chosen); i < n; i++ {
		p := (s.res.e.ctrl + i) % n
		if !s.pl[p].lost {
			s.res.chosen = objWord(i + 1)
			s.res.ask, s.res.askP = raVote, p
			return true
		}
	}
	return false
}

// voteStart is OpExt ExtVote: the first voter's ask, or nothing at all with
// an empty card ballot (gorge asks nobody and remembers nothing).
func (s *State) voteStart(k *prep, in *kop.Instr) bool {
	if in.C&voteCFixed == 0 && !s.voteBallot(k, in).any() {
		return true
	}
	if in.C&voteCFixed != 0 && voteChoiceCount(in) == 0 {
		return true
	}
	s.res.left, s.res.chosen = 0, 0
	return !s.voteNext()
}

// voteOptions poses one voter's ballot: one AkChoose per candidate, or one
// per fixed-list choice (source = the choice index, 0-based).
func (s *State) voteOptions(k *prep, d *Decision) {
	in := &s.prog(k)[s.res.pc]
	if in.C&voteCFixed != 0 {
		d.Sub = ChVoteChoice
		n := voteChoiceCount(in)
		for j := 0; j < n; j++ {
			// The source is the choice index (0-based): gorge encodes the
			// same pick as ObjID(j+1), whose canonical ref is the j-th
			// instance, so the two sides' options read identically.
			d.add(MakeAction(AkChoose, TCode(j), 0, 0))
		}
		return
	}
	d.Sub = ChVoteCard
	for m := s.voteBallot(k, in); m.any(); m = m.rest() {
		d.add(MakeAction(AkChoose, TCode(s.perms[m.first()].obj), 0, 0))
	}
}

// voteAnswer records a vote, then poses the next voter's ask or, once every
// vote is in, resolves the ballot and goes on.
func (s *State) voteAnswer(k *prep, a Action) {
	in := &s.prog(k)[s.res.pc]
	if in.C&voteCFixed != 0 {
		s.res.arr[s.res.left] = Obj(a.Source() + 1) // the choice index + 1
	} else {
		s.res.arr[s.res.left] = Obj(a.Source())
	}
	s.res.left++
	if s.voteNext() {
		return
	}
	if in.C&voteCFixed != 0 {
		s.voteFixedTally(k, in)
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.voteBallot(k, in), &ord)
	var votes [MaxPerm]uint8
	best := uint8(0)
	for i := 0; i < n; i++ {
		o := s.perms[ord[i]].obj
		for _, v := range s.res.arr[:s.res.left] {
			if v == o {
				votes[i]++
			}
		}
		best = max(best, votes[i])
	}
	var won slotMask
	for i := 0; i < n && best > 0; i++ {
		if votes[i] == best {
			s.remember(s.perms[ord[i]].obj)
			won = won.with(ord[i])
		}
	}
	if in.C&voteCExile != 0 && won.any() { // seat-ok: an instruction flag bit
		s.leaveAll(k, won, kop.ZoneExile) // one simultaneous departure (gorge's departure batch)
	}
	s.continueRes(k, 0)
}

// voteFixedTally counts the fixed-list votes and selects the winning (or tied)
// branch exactly as gorge's voteWinner/resolveVoteOutcomes: the first choice
// with the most votes wins, and a shared maximum runs the tie branch when the
// ability has one, else the first tied choice.
func (s *State) voteFixedTally(k *prep, in *kop.Instr) {
	n := voteChoiceCount(in)
	var counts [voteCMask + 1]uint8
	for _, v := range s.res.arr[:s.res.left] {
		j := int(v) - 1
		if j < 0 || j >= n {
			continue
		}
		counts[j]++
	}
	if in.C&voteCStore != 0 {
		// gorge resolveVoteOutcomes' StoreVoteNum branch: every option in
		// Choices$ order resolves with VoteNum bound to its own tally,
		// zero-tally options included. No winner is computed, so a
		// VoteTiedAbility$ branch (which lifting never emits in this mode)
		// never runs.
		if st := DebugVoteStats; st != nil {
			st.Rounds++
			st.StoreRounds++
			nz := 0
			for j := 0; j < n; j++ {
				if counts[j] != 0 {
					nz++
				}
			}
			if nz >= 2 {
				st.StoreMulti++
			}
		}
		s.resArm().voteStoreAll(counts, n)
		if DebugVoteNumWinnerOnly {
			// TestVoteStoreNumMutation: only the winning option (the first
			// maximum, gorge voteWinner) resolves.
			best := 0
			for j := 1; j < n; j++ {
				if counts[j] > counts[best] {
					best = j
				}
			}
			s.resArm().voteAll = false
			s.g.branch = uint8(best)
		}
		s.continueRes(k, 0)
		return
	}
	// gorge voteWinner: the first maximum, after every vote is counted.
	best := 0
	for j := 1; j < n; j++ {
		if counts[j] > counts[best] {
			best = j
		}
	}
	tied := 0
	for j := 0; j < n; j++ {
		if counts[j] == counts[best] {
			tied++
		}
	}
	pick := best
	if tied > 1 && in.C&voteCTie != 0 && !DebugVoteTieTakesFirst {
		pick = n // the extra VoteTiedAbility$ branch
	}
	if st := DebugVoteStats; st != nil {
		st.Rounds++
		if tied > 1 {
			st.Ties++
			if pick == n {
				st.TieBranch++
			}
		} else {
			st.NonTies++
		}
	}
	s.g.branch = uint8(pick)
	s.continueRes(k, 0)
}
