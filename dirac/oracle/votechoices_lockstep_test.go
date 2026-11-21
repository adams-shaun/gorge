package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// voteChoicesPodEntries is the votechoices fixture's four decks. The vote
// sorceries sit beside cheap creatures, burn and lands so every seat reaches
// a vote. The branch-refused spells (Galadriel, Trial of a Time Lord,
// Expropriate, Sail into the West, Split Decision) stay out of the decks but
// remain in the pool's cards.txt; Coercive Portal, Council Guardian and
// Magister of Worth lower but have no evidence decks yet.
func voteChoicesPodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Tyrant's Choice", Count: 4}, {Name: "Bite of the Black Rose", Count: 2},
			{Name: "Plea for Power", Count: 1}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3},
			{Name: "Giant Growth", Count: 2}, {Name: "Swamp", Count: 8},
			{Name: "Island", Count: 5}, {Name: "Forest", Count: 4},
			{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
		},
		{
			{Name: "Plea for Power", Count: 4}, {Name: "Bite of the Black Rose", Count: 2},
			{Name: "Tyrant's Choice", Count: 1}, {Name: "Giant Growth", Count: 2},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3},
			{Name: "Lightning Bolt", Count: 2}, {Name: "Island", Count: 8},
			{Name: "Swamp", Count: 5}, {Name: "Forest", Count: 4},
			{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
		},
		{
			{Name: "Bite of the Black Rose", Count: 4}, {Name: "Plea for Power", Count: 2},
			{Name: "Tyrant's Choice", Count: 1}, {Name: "Giant Growth", Count: 2},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3},
			{Name: "Lightning Bolt", Count: 2}, {Name: "Swamp", Count: 8},
			{Name: "Island", Count: 5}, {Name: "Forest", Count: 4},
			{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
		},
		{
			{Name: "Tyrant's Choice", Count: 3}, {Name: "Plea for Power", Count: 2},
			{Name: "Bite of the Black Rose", Count: 2}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Savannah Lions", Count: 3}, {Name: "Giant Growth", Count: 2},
			{Name: "Lightning Bolt", Count: 2}, {Name: "Swamp", Count: 7},
			{Name: "Island", Count: 6}, {Name: "Forest", Count: 4},
			{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
		},
	}
}

type voteChoicesPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newVoteChoicesPod(t *testing.T) voteChoicesPod {
	t.Helper()
	return newVoteChoicesPodOf(t, voteChoicesPodEntries())
}

func newVoteChoicesPodOf(t *testing.T, decks [4][]cardset.Entry) voteChoicesPod {
	t.Helper()
	reg := synthReg(t) // skips without the corpus, before any table load
	set, err := cardset.RawByName("votechoices")
	if err != nil {
		t.Fatal(err)
	}
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return voteChoicesPod{reg: reg, set: set, tbl: tbl, decks: decks}
}

func voteChoicesExpand(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for j := 0; j < e.Count; j++ {
			out = append(out, e.Name)
		}
	}
	return out
}

// voteChoicesPlayFour plays one four-seat seed and returns the run and the
// fixed-list vote counters.
func (p voteChoicesPod) playFour(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
	t.Helper()
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards:     [2][]string{voteChoicesExpand(p.decks[0]), voteChoicesExpand(p.decks[1])},
		MoreDecks: []string{"x2", "x3"},
		MoreCards: [][]string{voteChoicesExpand(p.decks[2]), voteChoicesExpand(p.decks[3])},
		Seed:      seed, PassElision: true, Surface: "full"}
	return p.run(t, r, seed, maxTurns)
}

// voteChoicesPlayTwo plays one two-seat seed over the first two decks.
func (p voteChoicesPod) playTwo(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
	t.Helper()
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{voteChoicesExpand(p.decks[0]), voteChoicesExpand(p.decks[1])},
		Seed:  seed, PassElision: true, Surface: "full"}
	return p.run(t, r, seed, maxTurns)
}

func (p voteChoicesPod) run(t *testing.T, r *Repro, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
	t.Helper()
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	st := &dirac.VoteStats{}
	dirac.DebugVoteStats = st
	defer func() { dirac.DebugVoteStats = nil }()
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	return res, *st
}

func voteChoicesFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

func voteChoicesStop(t *testing.T, seed uint64, res Result) {
	t.Helper()
	if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
	}
}

// TestVoteChoicesFourSeatLockstep plays four-seat pods of the votechoices
// fixture. Every seat votes in turn order from the controller, the winning
// branch (or the tie branch) runs, and over the seeds both a tie and a
// unique winner occur.
func TestVoteChoicesFourSeatLockstep(t *testing.T) {
	p := newVoteChoicesPod(t)
	var sum dirac.VoteStats
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8} {
		res, st := p.playFour(t, seed, 40)
		if res.Div != nil {
			voteChoicesFail(t, seed, res)
		}
		voteChoicesStop(t, seed, res)
		sum.Rounds += st.Rounds
		sum.Ties += st.Ties
		sum.NonTies += st.NonTies
		sum.TieBranch += st.TieBranch
		t.Logf("seed %d: %d decisions, over %v %s; %d vote(s), %d tie(s), %d unique", seed, res.Decisions, res.Over, res.Stall,
			st.Rounds, st.Ties, st.NonTies)
	}
	if sum.Rounds == 0 {
		t.Fatal("no fixed-list vote resolved")
	}
	if sum.Ties == 0 || sum.NonTies == 0 {
		t.Fatalf("votes %d: ties %d, unique %d; want both above zero", sum.Rounds, sum.Ties, sum.NonTies)
	}
	if sum.TieBranch == 0 {
		t.Fatal("no tie ran a VoteTiedAbility$ branch")
	}
}

// TestVoteChoicesTwoSeatLockstep is the duel form: two voters, still asking
// in gorge's order and resolving the same branch.
func TestVoteChoicesTwoSeatLockstep(t *testing.T) {
	p := newVoteChoicesPod(t)
	var sum dirac.VoteStats
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, st := p.playTwo(t, seed, 40)
		if res.Div != nil {
			voteChoicesFail(t, seed, res)
		}
		voteChoicesStop(t, seed, res)
		sum.Rounds += st.Rounds
		sum.Ties += st.Ties
		sum.NonTies += st.NonTies
		t.Logf("seed %d: %d decisions, over %v %s; %d vote(s), %d tie(s), %d unique", seed, res.Decisions, res.Over, res.Stall,
			st.Rounds, st.Ties, st.NonTies)
	}
	if sum.Rounds == 0 {
		t.Fatal("no fixed-list vote resolved")
	}
}

// TestVoteChoicesPinnedSeedFires pins a seed where a fixed-list vote actually
// resolved, so a green lockstep is not a game where nothing happened.
func TestVoteChoicesPinnedSeedFires(t *testing.T) {
	p := newVoteChoicesPod(t)
	res, st := p.playFour(t, 1, 40)
	if res.Div != nil {
		voteChoicesFail(t, 1, res)
	}
	voteChoicesStop(t, 1, res)
	if st.Rounds == 0 {
		t.Fatalf("seed 1: no fixed-list vote resolved (%+v)", st)
	}
	t.Logf("seed 1: %d vote(s), %d tie(s), %d unique", st.Rounds, st.Ties, st.NonTies)
}

// TestVoteChoicesTieMutation is the mutation test: resolving the first tied
// option instead of the VoteTiedAbility$ branch must diverge on the seed
// where a tie runs that branch.
func TestVoteChoicesTieMutation(t *testing.T) {
	p := newVoteChoicesPod(t)
	var seed uint64
	for s := uint64(1); s <= 16; s++ {
		_, st := p.playFour(t, s, 40)
		if st.TieBranch > 0 {
			seed = s
			break
		}
	}
	if seed == 0 {
		t.Fatal("no seed ran a VoteTiedAbility$ branch")
	}
	dirac.DebugVoteTieTakesFirst = true
	defer func() { dirac.DebugVoteTieTakesFirst = false }()
	res, _ := p.playFour(t, seed, 40)
	if res.Div == nil {
		t.Fatalf("seed %d: resolving the first tied option did not diverge", seed)
	}
	t.Logf("seed %d: mutation diverged at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
}
