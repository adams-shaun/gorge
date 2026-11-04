package oracle

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Oko, the Ringleader's -5 copy-all sweep over a board with an
// Aura whose copy asks (CR 303.4f). Stop Cold is an object-enchanting Aura
// with more than one legal host, so its copy's host ask must be posed exactly
// where gorge poses it -- after the earlier sources' copies entered, with the
// sweep's remaining sources still unentered -- and the answer applied before
// the sweep resumes. Deck B's only attackers are 0-powered, so Oko is not
// killed in combat on the way to the -5.

var kcopyallDecks = [2][]cardset.Entry{
	{
		{Name: "Oko, the Ringleader", Count: 4}, {Name: "Stop Cold", Count: 3},
		{Name: "Armored Armadillo", Count: 4}, {Name: "Deepmuck Desperado", Count: 4},
		{Name: "Inventive Wingsmith", Count: 4},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Armored Armadillo", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 12}, {Name: "Plains", Count: 12},
	},
}

// kcopyallCounts is one run's observed branches, read off gorge's event log
// and the probe's answers.
type kcopyallCounts struct {
	tokens   atomic.Int64 // Stop Cold copies minted by the sweep
	attached atomic.Int64 // of those, copies that entered attached
	detached atomic.Int64 // of those, copies that entered with no host
	asks     atomic.Int64 // host asks the probe answered
	seed     uint64       // seed of the run in progress
}

// kcopyallFinish counts the sweep's Stop Cold copies off the finished game's
// event log.
func kcopyallFinish(t *testing.T, c *kcopyallCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		// A CopyToken event names the SOURCE object; the minted copy shows
		// up as the next MoveZone (a genuine entry, or the answered
		// as-enters ask's entry). Pair each sweep's Stop Cold source with
		// the token that follows it, then an Attach of that token counts.
		var entered []state.ObjID
		tokAt := map[state.ObjID]int{}
		pending, tok := false, state.ObjID(0)
		copyAt := map[state.ObjID]int{}
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			switch e.Kind {
			case events.CopyToken:
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Stop Cold" {
					c.tokens.Add(1)
					pending = true
				}
			case events.MoveZone:
				if pending {
					tok = e.Obj
					tokAt[tok] = i
					entered = append(entered, tok)
					pending = false
				}
			case events.Attach:
				copyAt[e.Obj] = i
			}
		}
		nAtt := 0
		for _, id := range entered {
			if j, ok := copyAt[id]; ok && j > tokAt[id] {
				nAtt++
			}
		}
		c.attached.Add(int64(nAtt))
		c.detached.Add(int64(len(entered) - nAtt))
	}
}

// kcopyallProbe forces Oko's path: cast Oko, activate +1 until -5 is offered,
// -5, and cast Stop Cold whenever it is offered. Every host ask for a Stop
// Cold token is counted; the answer alternates by seed between the first
// offered host and the policy's own pick. The ask offers hosts only (CR
// 303.4f): a Stop Cold copy enters detached only when no legal host exists,
// the sweep's 0-candidate skip.
func kcopyallProbe(t *testing.T, c *kcopyallCounts, seed uint64, base Policy) Policy {
	t.Helper()
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil {
			return base.Pick(p)
		}
		for _, a := range p.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			o, ok := p.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := p.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			if ob.Face().Name == "Oko, the Ringleader" || ob.Face().Name == "Stop Cold" {
				return a
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() != ActActivate {
				continue
			}
			o, ok := p.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := p.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil || ob.Face().Name != "Oko, the Ringleader" {
				continue
			}
			if o.Loyalty.Delta == -5 {
				return a
			}
		}
		// The ETB host ask of a sweep-minted Aura copy surfaces as the
		// canonical decision once dirac reaches it (pre-fix the kernel
		// stalls first, so it never appears).
		if d := p.G.E.Pending(); d != nil && d.ResumeKind == "etb" && len(d.Options) > 0 &&
			d.Options[0].Kind == "enchant" {
			if ob := p.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil &&
				ob.Face().Name == "Stop Cold" && ob.IsToken {
				c.asks.Add(1)
			}
		}
		if pd := p.G.Pending(); pd != nil && (p.D.Kind == DChoose || p.D.Kind == DTarget) {
			if ob := p.G.E.G.Obj(pd.Source); ob != nil && ob.Face() != nil &&
				ob.Face().Name == "Stop Cold" && ob.IsToken {
				c.asks.Add(1)
				if (seed+uint64(p.Seq))%2 == 0 {
					for _, x := range p.D.Options {
						if x.Kind() != ActCast {
							return x
						}
					}
				}
			}
		}
		return base.Pick(p)
	})
}

// kcopyallRun plays seeds 1..n; with report each divergence or stall is an
// error, without it the first divergence or stall returns diverged.
func kcopyallRun(t *testing.T, n uint64, report bool, c *kcopyallCounts) (diverged bool, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set := cardsettest.Synth(t, "std-otj")
	var decks [2][]*cards.Card
	for s, d := range kcopyallDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], card)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kcopyallDecks[0], kcopyallDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		if c != nil {
			c.seed = seed
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcopyall-a", "kcopyall-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := Policy(HeuristicPolicy(seed))
		if seed%3 == 0 {
			base = RandomPolicy(seed)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 40}
		if c != nil {
			rc.Finish = kcopyallFinish(t, c)
		}
		res := Lockstep(m, seed, k, kcopyallProbe(t, c, seed, base), rc)
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			diverged = true
			if !report {
				t.Logf("mutant diverged at seed %d: %s", seed, d)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

// TestCopyAllAskLockstep: 0 divergences, 0 stalls, and the branch minima.
func TestCopyAllAskLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var c kcopyallCounts
	div, stalls := kcopyallRun(t, 96, true, &c)
	t.Logf("96 seeds, %d divergences, %d stalls; stop cold tokens %d (attached %d, detached %d), host asks %d",
		b2i(div), stalls, c.tokens.Load(), c.attached.Load(), c.detached.Load(), c.asks.Load())
	if div || stalls > 0 {
		return
	}
	for _, f := range []struct {
		s        string
		got, min int64
	}{
		{"stop cold copies minted", c.tokens.Load(), 10},
		{"stop cold copies that attached", c.attached.Load(), 5},
		{"sweep host asks answered", c.asks.Load(), 10},
	} {
		if f.got < f.min {
			t.Errorf("%s: %d, want at least %d", f.s, f.got, f.min)
		}
	}
}
