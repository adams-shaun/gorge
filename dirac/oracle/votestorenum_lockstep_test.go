package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// voteStoreNumPodEntries is the votestorenum fixture's four decks. The eight
// Council's dilemma carriers sit beside cheap creatures, burn and lands so
// every seat reaches a vote; each seat's colour pair can cast its share.
func voteStoreNumPodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Lieutenants of the Guard", Count: 2}, {Name: "Orchard Elemental", Count: 2},
			{Name: "Selvala's Stampede", Count: 2}, {Name: "Emissary Green", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
			{Name: "Giant Growth", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Messenger Jays", Count: 2}, {Name: "Travel Through Caradhras", Count: 2},
			{Name: "Emissary Green", Count: 2}, {Name: "Orchard Elemental", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
			{Name: "Giant Growth", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Island", Count: 9}, {Name: "Forest", Count: 11},
		},
		{
			{Name: "Fateful Tempest", Count: 3}, {Name: "Capital Punishment", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
			{Name: "Giant Growth", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11},
		},
		{
			{Name: "Lieutenants of the Guard", Count: 2}, {Name: "Messenger Jays", Count: 2},
			{Name: "Selvala's Stampede", Count: 1}, {Name: "Travel Through Caradhras", Count: 1},
			{Name: "Fateful Tempest", Count: 1}, {Name: "Capital Punishment", Count: 1},
			{Name: "Orchard Elemental", Count: 1}, {Name: "Emissary Green", Count: 1},
			{Name: "Grizzly Bears", Count: 2}, {Name: "Savannah Lions", Count: 2},
			{Name: "Giant Growth", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Plains", Count: 8}, {Name: "Forest", Count: 7},
			{Name: "Island", Count: 5}, {Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 1},
		},
	}
}

type voteStoreNumPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newVoteStoreNumPod(t *testing.T) voteStoreNumPod {
	t.Helper()
	return newVoteStoreNumPodOf(t, voteStoreNumPodEntries())
}

func newVoteStoreNumPodOf(t *testing.T, decks [4][]cardset.Entry) voteStoreNumPod {
	t.Helper()
	reg := synthReg(t) // skips without the corpus, before any table load
	set, err := cardset.RawByName("votestorenum")
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
	return voteStoreNumPod{reg: reg, set: set, tbl: tbl, decks: decks}
}

func voteStoreNumExpand(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for j := 0; j < e.Count; j++ {
			out = append(out, e.Name)
		}
	}
	return out
}

func (p voteStoreNumPod) playFour(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
	t.Helper()
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards:     [2][]string{voteStoreNumExpand(p.decks[0]), voteStoreNumExpand(p.decks[1])},
		MoreDecks: []string{"x2", "x3"},
		MoreCards: [][]string{voteStoreNumExpand(p.decks[2]), voteStoreNumExpand(p.decks[3])},
		Seed:      seed, PassElision: true, Surface: "full"}
	return p.run(t, r, seed, maxTurns)
}

func (p voteStoreNumPod) playTwo(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
	t.Helper()
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{voteStoreNumExpand(p.decks[0]), voteStoreNumExpand(p.decks[1])},
		Seed:  seed, PassElision: true, Surface: "full"}
	return p.run(t, r, seed, maxTurns)
}

func (p voteStoreNumPod) run(t *testing.T, r *Repro, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
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

func voteStoreNumFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

func voteStoreNumStop(t *testing.T, seed uint64, res Result) {
	t.Helper()
	if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
	}
}

// TestVoteStoreNumFourSeatLockstep plays four-seat pods of the votestorenum
// fixture: after the last voter every Choices$ option resolves in order with
// VoteNum bound to that option's tally, zero-tally options included, and the
// two sides agree decision by decision. Both a resolution with two nonzero
// tallies and a zero-tally option occur.
func TestVoteStoreNumFourSeatLockstep(t *testing.T) {
	p := newVoteStoreNumPod(t)
	var rounds, multi int
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8} {
		res, st := p.playFour(t, seed, 40)
		if res.Div != nil {
			voteStoreNumFail(t, seed, res)
		}
		voteStoreNumStop(t, seed, res)
		rounds += st.StoreRounds
		multi += st.StoreMulti
		t.Logf("seed %d: %d decisions, over %v %s; %d StoreVoteNum round(s), %d with two nonzero tallies", seed, res.Decisions, res.Over, res.Stall, st.StoreRounds, st.StoreMulti)
	}
	if rounds == 0 {
		t.Fatal("no StoreVoteNum vote resolved")
	}
	if multi == 0 {
		t.Fatalf("StoreVoteNum rounds %d: none had two nonzero tallies", rounds)
	}
}

// TestVoteStoreNumTwoSeatLockstep is the duel form: two voters, every option
// still resolving with its own tally.
func TestVoteStoreNumTwoSeatLockstep(t *testing.T) {
	p := newVoteStoreNumPod(t)
	var rounds int
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, st := p.playTwo(t, seed, 40)
		if res.Div != nil {
			voteStoreNumFail(t, seed, res)
		}
		voteStoreNumStop(t, seed, res)
		rounds += st.StoreRounds
		t.Logf("seed %d: %d decisions, over %v %s; %d StoreVoteNum round(s)", seed, res.Decisions, res.Over, res.Stall, st.StoreRounds)
	}
	if rounds == 0 {
		t.Fatal("no StoreVoteNum vote resolved")
	}
}

// TestVoteStoreNumMutation is the mutation test: resolving only the winning
// option (DebugVoteNumWinnerOnly) must diverge on a seed whose vote had two
// nonzero tallies, because the losing option's body no longer runs.
func TestVoteStoreNumMutation(t *testing.T) {
	p := newVoteStoreNumPod(t)
	var seed uint64
	for s := uint64(1); s <= 16; s++ {
		res, st := p.playFour(t, s, 40)
		if res.Div != nil {
			voteStoreNumFail(t, s, res)
		}
		if st.StoreMulti > 0 {
			seed = s
			break
		}
	}
	if seed == 0 {
		t.Fatal("no seed resolved a StoreVoteNum vote with two nonzero tallies")
	}
	dirac.DebugVoteNumWinnerOnly = true
	defer func() { dirac.DebugVoteNumWinnerOnly = false }()
	res, _ := p.playFour(t, seed, 40)
	if res.Div == nil {
		t.Fatalf("seed %d: resolving only the winning option did not diverge", seed)
	}
	t.Logf("seed %d: mutation diverged at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
}

// TestVoteStoreNumPinnedSeedFires pins a seed where a StoreVoteNum vote
// actually resolved with two nonzero tallies, so a green lockstep is not a
// game where nothing happened.
func TestVoteStoreNumPinnedSeedFires(t *testing.T) {
	p := newVoteStoreNumPod(t)
	res, st := p.playFour(t, 1, 40)
	if res.Div != nil {
		voteStoreNumFail(t, 1, res)
	}
	voteStoreNumStop(t, 1, res)
	if st.StoreRounds == 0 {
		t.Fatalf("seed 1: no StoreVoteNum vote resolved (%+v)", st)
	}
	if st.StoreMulti == 0 {
		t.Fatalf("seed 1: no vote had two nonzero tallies (%+v)", st)
	}
	t.Logf("seed 1: %d StoreVoteNum round(s), %d with two nonzero tallies", st.StoreRounds, st.StoreMulti)
}
