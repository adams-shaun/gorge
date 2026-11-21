package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// triggeredVoteDecks is the votechoices fixture's two decks for the ETB
// triggered fixed-list vote carriers that lower since gorge stopped posing a
// placement mode for a triggered Vote (rules/trigger_queue.go
// askTriggerModes returns false for API Vote): Coercive Portal, Council
// Guardian and Magister of Worth enter and put their will-of-the-council
// trigger on the stack, and the ballot is asked at the trigger's own
// resolution. Both decks carry all three beside their mana and bear fodder;
// each is 40.
func triggeredVoteDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Council Guardian", Count: 4}, {Name: "Magister of Worth", Count: 4},
			{Name: "Coercive Portal", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
		},
		{
			{Name: "Council Guardian", Count: 4}, {Name: "Magister of Worth", Count: 4},
			{Name: "Coercive Portal", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
		},
	}
}

type triggeredVotePod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [2][]cardset.Entry
}

func newTriggeredVotePod(t *testing.T) triggeredVotePod {
	t.Helper()
	reg := synthReg(t) // skips without the corpus, before any table load
	set, err := cardset.RawByName("votechoices")
	if err != nil {
		t.Fatal(err)
	}
	lists := triggeredVoteDecks()
	for i, d := range lists {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
	}
	tbl, _, err := set.MatchupN(lists[:])
	if err != nil {
		t.Fatal(err)
	}
	return triggeredVotePod{reg: reg, set: set, tbl: tbl, decks: lists}
}

func (p triggeredVotePod) playTwo(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.VoteStats) {
	t.Helper()
	d := triggeredVoteDecks()
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{voteChoicesExpand(d[0]), voteChoicesExpand(d[1])},
		Seed:  seed, PassElision: true, Surface: "full"}
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

// TestTriggeredVoteLockstep plays two-seat pods whose every creature is a
// triggered fixed-list vote carrier: the ETB trigger goes on the stack with
// no mode ask (gorge askTriggerModes returns false for API Vote), each player
// votes in turn order at the trigger's resolution, and the winning branch (or
// the tie branch) runs identically on both sides.
func TestTriggeredVoteLockstep(t *testing.T) {
	p := newTriggeredVotePod(t)
	var sum dirac.VoteStats
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, st := p.playTwo(t, seed, 40)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q\nref %q\nker %q\n%s last actions %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.Rounds += st.Rounds
		sum.Ties += st.Ties
		sum.NonTies += st.NonTies
		sum.TieBranch += st.TieBranch
		t.Logf("seed %d: %d decisions, over %v %s; %d vote(s), %d tie(s), %d unique", seed, res.Decisions, res.Over, res.Stall,
			st.Rounds, st.Ties, st.NonTies)
	}
	if sum.Rounds == 0 {
		t.Fatal("no triggered vote resolved")
	}
	if sum.Ties == 0 || sum.NonTies == 0 {
		t.Fatalf("triggered votes %d: ties %d, unique %d; want both above zero", sum.Rounds, sum.Ties, sum.NonTies)
	}
	if sum.TieBranch == 0 {
		t.Fatal("no tie ran a VoteTiedAbility$ branch")
	}
}
