package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The target-player sweep lockstep: PutCounterAll and
// UntapAll scoped by ValidTgts$ Player (Twisted Fates, Requisition Raid,
// Practiced Offense, Rustler Rampage), PutCounterAll's second batch (Claim
// the Kingdom's plan counter, Kheru Goldkeeper's flying counter, Sagu
// Pummeler's reach counter) and Herd Baloth's "one or more +1/+1 counters"
// listener, which makes the two-batch action observable. The expected
// answer at every decision is gorge's.

var ksweeptgtDecks = [2][]cardset.Entry{
	// A: white/black sweep spells and cheap white/green bodies.
	{
		{Name: "Twisted Fates", Count: 3}, {Name: "Requisition Raid", Count: 4}, {Name: "Practiced Offense", Count: 4},
		{Name: "Rustler Rampage", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Savannah Lions", Count: 3},
		{Name: "Isamaru, Hound of Konda", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 3},
	},
	// B: the green/blue/black sweep payoffs, a Renew body each and the
	// counter-added listener.
	{
		{Name: "Kheru Goldkeeper", Count: 4}, {Name: "Sagu Pummeler", Count: 4}, {Name: "Claim the Kingdom", Count: 4},
		{Name: "Herd Baloth", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 4},
	},
}

// ksweepStats counts the sweep resolutions and their target seat. A sweep
// is one PutCounterAll or UntapAll resolution that touched a permanent;
// "opp" counts a sweep that touched at least one permanent the resolving
// player did not control.
type ksweepStats struct {
	putSweeps, putOpp     int
	untapSweeps, untapOpp int
	claimLandfall         int
	renewActs             int
}

// ksweepTally reads the gorge event log. It attributes a CounterChange run
// to the resolving source (gorge's CounterChange has no source field) and a
// contiguous run of CounterChange or Untap events to one sweep batch.
type ksweepTally struct {
	evAt      int
	src       string
	srcCtrl   state.PlayerID
	putOpen   bool
	putOpp    bool
	untapOpen bool
	untapOpp  bool
}

func (p *ksweepTally) closeRuns(st *ksweepStats) {
	if p.putOpen {
		if p.putOpp {
			st.putOpp++
		}
		p.putOpen, p.putOpp = false, false
	}
	if p.untapOpen {
		if p.untapOpp {
			st.untapOpp++
		}
		p.untapOpen, p.untapOpp = false, false
	}
}

func (p *ksweepTally) isSweepSource() bool {
	switch p.src {
	case "Twisted Fates", "Requisition Raid", "Practiced Offense", "Claim the Kingdom", "Kheru Goldkeeper", "Sagu Pummeler":
		return true
	}
	return false
}

func (p *ksweepTally) scan(ref *GorgeSide, st *ksweepStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.CounterChange && e.Kind != events.Untap {
			p.closeRuns(st)
		}
		switch e.Kind {
		case events.Resolve:
			p.src, p.srcCtrl = chName(g, e.Obj), e.Player
		case events.Priority:
			p.src, p.srcCtrl = "", 0
		case events.CounterChange:
			if !p.putOpen {
				if !p.isSweepSource() || e.Amount <= 0 {
					continue
				}
				p.putOpen = true
				st.putSweeps++
			}
			if o := g.Obj(e.Obj); o != nil && o.Controller != p.srcCtrl {
				p.putOpp = true
			}
		case events.Untap:
			if !p.untapOpen {
				if p.src != "Rustler Rampage" {
					continue
				}
				p.untapOpen = true
				st.untapSweeps++
			}
			if o := g.Obj(e.Obj); o != nil && o.Controller != p.srcCtrl {
				p.untapOpp = true
			}
		case events.TriggerPush:
			if e.Amount == 0 && chName(g, e.Obj) == "Claim the Kingdom" {
				st.claimLandfall++
			}
		case events.AbilityPush:
			switch chName(g, e.Obj) {
			case "Kheru Goldkeeper", "Sagu Pummeler":
				st.renewActs++
			}
		}
	}
	p.evAt = len(evs)
}

// ksweepPolicy prefers casting, activating and playing lands (so the
// carriers are reached and the mana develops), else takes a random option;
// it feeds the gorge event log to the tally on every pick.
func ksweepPolicy(seed uint64, st *ksweepStats, tl *ksweepTally) Policy {
	rnd := RandomPolicy(seed)
	var buf, all []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		buf, all = buf[:0], all[:0]
		for _, a := range c.D.Options {
			all = append(all, a)
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			buf = all
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runSweeptgt plays the constructed matchup for seeds games and returns the
// stats, divergences and stalls. mutate widens the kernel's F2TgtPlayerCtrl
// to every seat; the lockstep must then diverge.
func runSweeptgt(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st ksweepStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sweeptgt")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ksweeptgtDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	old := dirac.DebugTgtPlayerCtrlAny
	dirac.DebugTgtPlayerCtrlAny = mutate
	defer func() { dirac.DebugTgtPlayerCtrlAny = old }()
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		tbl, _, _, err := set.Matchup(ksweeptgtDecks[0], ksweeptgtDecks[1])
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMatchup(reg, [2]string{"ksweeptgt-a", "ksweeptgt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		var tl ksweepTally
		res := LockstepLed(m, seed, g, k, ksweepPolicy(seed, &st, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		tl.scan(g, &st)
		CloseSide(k)
		games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestSweeptgtLockstep(t *testing.T) {
	t.Parallel()
	games, divs, stalls, st := runSweeptgt(t, 400, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("sweeps: PutCounterAll %d (opponent %d), UntapAll %d (opponent %d); Claim the Kingdom landfalls %d; renew activations %d",
		st.putSweeps, st.putOpp, st.untapSweeps, st.untapOpp, st.claimLandfall, st.renewActs)
	if divs != 0 || stalls != 0 {
		t.Fatalf("lockstep: %d divergences, %d stalls", divs, stalls)
	}
	if st.putSweeps < 10 || st.untapSweeps < 10 {
		t.Fatalf("too few scoped sweeps: PutCounterAll %d, UntapAll %d (want >= 10 each)", st.putSweeps, st.untapSweeps)
	}
	if st.putOpp < 3 || st.untapOpp < 3 {
		t.Fatalf("too few opponent-aimed sweeps: PutCounterAll %d, UntapAll %d (want >= 3 each)", st.putOpp, st.untapOpp)
	}
	if st.claimLandfall < 5 {
		t.Fatalf("Claim the Kingdom landfall triggers %d, want >= 5", st.claimLandfall)
	}
	if st.renewActs < 5 {
		t.Fatalf("renew activations %d, want >= 5", st.renewActs)
	}
}

// TestSweeptgtMutation proves the pool reaches the scoped sweeps: when the
// kernel's F2TgtPlayerCtrl matches every seat, some seed diverges.
func TestSweeptgtMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	_, divs, _, st := runSweeptgt(t, 128, true)
	t.Logf("mutation: %d divergences; %+v", divs, st)
	if divs == 0 {
		t.Fatal("no seed diverged with F2TgtPlayerCtrl widened to every seat: the pool does not exercise the scoped sweep")
	}
}
