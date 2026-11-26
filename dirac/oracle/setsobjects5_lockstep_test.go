package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The setsobjects5 fixture pool's lockstep runs, at two seats and at four (a
// Commander pod: every player votes):
//
//   - Vote Zone$ Graveyard (kgen lower_vote.go cardVote, the kernel's
//     act_vote.go voteGraveTally): Custodi Squire's voters each pick an
//     artifact, creature or enchantment card in its controller's graveyard,
//     and the most-voted return to hand. The scenario is reached when a
//     graveyard ballot of two or more cards is posed. The mutation leaves the
//     most-voted in the graveyard and must diverge.
//   - ChangeZone LeaveBattlefield$ Exile (kgen lower_zone.go, the kernel's
//     zone_leaveexile.go): Isareth the Awakener's reflexive return and From
//     the Catacombs put a creature card onto the battlefield with an
//     exile-instead promise. The scenario is reached when each card
//     registers a promise and some promised creature then leaves the
//     battlefield (exiled instead). The mutation ignores the promise and
//     must diverge.
//
// The expected answer at every decision is gorge's.

type so5Tally struct {
	asks     map[string]int // the counted asks, by name
	lastSeen string         // the last counted decision, so a re-posed one counts once
}

// so5Policy prefers casts, activations and land plays (three decisions in
// four), counting the asks see reports for each pending gorge decision.
func so5Policy(seed uint64, tl *so5Tally, see func(g *GorgeSide, d *decision.Decision, tl *so5Tally)) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			if d := g.E.Pending(); d != nil {
				key := fmt.Sprintf("%d/%d/%d/%s", c.Seq, d.Player, d.Source, d.ResumeKind)
				if key != tl.lastSeen {
					tl.lastSeen = key
					see(g, d, tl)
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || c.Seq%4 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runSO5 plays seeds games of list at every seat over the setsobjects5 pool;
// edit (when non-nil) sees the table before the games; mutated runs may
// diverge or stall.
func runSO5(t *testing.T, seats, seeds int, list []cardset.Entry, mutated bool, edit func(tbl *kop.Table), see func(g *GorgeSide, d *decision.Decision, tl *so5Tally)) (fixtureRun, *so5Tally) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("setsobjects5")
	if err != nil {
		t.Fatal(err)
	}
	var deck []string
	n := 0
	for _, e := range list {
		n += e.Count
		for range e.Count {
			deck = append(deck, e.Name)
		}
	}
	if n != 40 {
		t.Fatalf("deck: %d cards", n)
	}
	lists := make([][]cardset.Entry, seats)
	for i := range lists {
		lists[i] = list
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(tbl)
	}
	tl := &so5Tally{asks: map[string]int{}}
	var run fixtureRun
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"so5-0", "so5-1"}, Cards: [2][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
		for s := 2; s < seats; s++ {
			r.MoreDecks = append(r.MoreDecks, fmt.Sprintf("so5-%d", s))
			r.MoreCards = append(r.MoreCards, deck)
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 25
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, so5Policy(seed, tl, see), rc)
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.StallUnmodelled {
			if !mutated {
				t.Fatalf("%d seats, seed %d: kernel stopped: %s", seats, seed, res.Stall)
			}
			run.stalls++
		}
		if d := res.Div; d != nil {
			if !mutated {
				t.Fatalf("%d seats, seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seats, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			run.divs++
			if run.first == "" {
				run.first = fmt.Sprintf("seed %d at %d: %s %s", seed, d.At, d.What, d.Field)
			}
		}
	}
	return run, tl
}

func so5VoteGraveDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Custodi Squire", Count: 8}, {Name: "Ornithopter", Count: 4}, {Name: "Elite Vanguard", Count: 6},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Plains", Count: 9}, {Name: "Mountain", Count: 7},
	}
}

// so5SeeVoteGrave counts Custodi Squire's graveyard ballots of two or more
// cards ("ballot2").
func so5SeeVoteGrave(g *GorgeSide, d *decision.Decision, tl *so5Tally) {
	if d.Kind != decision.KChoose || d.ResumeKind != "vote" || len(d.Options) < 2 || d.Options[0].Kind != "vote_card" {
		return
	}
	if src := g.E.G.Obj(d.Source); src != nil && src.Face() != nil && src.Face().Name == "Custodi Squire" {
		tl.asks["ballot2"]++
	}
}

func TestSetsObjects5VoteGraveLockstep(t *testing.T) {
	for _, seats := range []int{2, 4} {
		t.Run(fmt.Sprintf("seats%d", seats), func(t *testing.T) {
			run, tl := runSO5(t, seats, 16, so5VoteGraveDeck(), false, nil, so5SeeVoteGrave)
			t.Logf("%d seats, %d games: %v", seats, run.games, tl.asks)
			if tl.asks["ballot2"] == 0 {
				t.Errorf("%d seats: no graveyard ballot of two or more cards was posed", seats)
			}
		})
	}
	t.Run("mutation-no-return", func(t *testing.T) {
		dirac.DebugVoteGraveNoReturn = true
		defer func() { dirac.DebugVoteGraveNoReturn = false }()
		run, _ := runSO5(t, 2, 16, so5VoteGraveDeck(), true, nil, so5SeeVoteGrave)
		run.requireDivergence(t, "the most-voted graveyard cards left in the graveyard")
	})
}

func so5LeaveExileDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Isareth the Awakener", Count: 7}, {Name: "From the Catacombs", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Lightning Bolt", Count: 7}, {Name: "Swamp", Count: 11}, {Name: "Mountain", Count: 7},
	}
}

func so5NoSee(*GorgeSide, *decision.Decision, *so5Tally) {}

// runSO5LeaveExile plays the leave-exile deck, counting the promises
// registered by source card and the distinct objects exiled instead.
func runSO5LeaveExile(t *testing.T, seats, seeds int, mutate bool) (fixtureRun, map[string]int, int) {
	t.Helper()
	regs := map[string]int{}
	hits := map[dirac.Obj]bool{}
	var tbl *kop.Table
	dirac.DebugLeaveExileHook = &struct {
		Reg func(c dirac.CardID)
		Hit func(o dirac.Obj)
	}{
		Reg: func(c dirac.CardID) { regs[tbl.Cards[c].Name]++ },
		Hit: func(o dirac.Obj) { hits[o] = true },
	}
	defer func() { dirac.DebugLeaveExileHook = nil }()
	if mutate {
		dirac.DebugNoLeaveExile = true
		defer func() { dirac.DebugNoLeaveExile = false }()
	}
	run, _ := runSO5(t, seats, seeds, so5LeaveExileDeck(), mutate, func(t *kop.Table) { tbl = t }, so5NoSee)
	return run, regs, len(hits)
}

func TestSetsObjects5LeaveExileLockstep(t *testing.T) {
	for _, seats := range []int{2, 4} {
		t.Run(fmt.Sprintf("seats%d", seats), func(t *testing.T) {
			run, regs, hits := runSO5LeaveExile(t, seats, 16, false)
			t.Logf("%d seats, %d games: promises %v, exiled instead %d", seats, run.games, regs, hits)
			for _, name := range []string{"Isareth the Awakener", "From the Catacombs"} {
				if regs[name] == 0 {
					t.Errorf("%d seats: %s registered no exile-instead promise", seats, name)
				}
			}
			if hits == 0 {
				t.Errorf("%d seats: no promised creature left the battlefield", seats)
			}
		})
	}
	t.Run("mutation-no-promise", func(t *testing.T) {
		run, _, _ := runSO5LeaveExile(t, 2, 16, true)
		run.requireDivergence(t, "the exile-instead promise ignored")
	})
}
