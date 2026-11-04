package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed mirror matchups over the sisko pool. Captain
// Benjamin Sisko's five-colour ability exiles the top five cards of the
// activator's library, and the activator may cast any number of them free
// when their mana value is at most the Officer count -- a bound named by the
// SVar Y (a count of the controller's Officers), not X. The deck mixes
// Starfleet Crew (a vanilla Officer, to move the count), spells of mana
// value one to four, and lands (never cast).

var siskoDeck = []cardset.Entry{
	{Name: "Captain Benjamin Sisko", Count: 5}, {Name: "Starfleet Crew", Count: 5},
	{Name: "Shock", Count: 2}, {Name: "Opt", Count: 2}, {Name: "Grizzly Bears", Count: 3},
	{Name: "Divination", Count: 3}, {Name: "Hill Giant", Count: 3},
	{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
	{Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
}

type siskoTally struct {
	asks      int         // free-cast asks put for Sisko's ability
	picks     int         // asks answered with a cast
	declines  int         // asks answered without one
	officers  map[int]int // asks by the Officer count at the ask
	cut       int         // asks where the exile held a nonland card above the bound
	landSeen  int         // asks where the exile held a land
	multiPick int         // asks answered with a cast while another card was still castable
}

func siskoOfficers(g *state.Game, pl state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, pl) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Controller != pl {
			continue
		}
		for _, ty := range o.Face().Types {
			if strings.EqualFold(ty, "Officer") {
				n++
				break
			}
		}
	}
	return n
}

func siskoPolicy(seed uint64, t *siskoTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		hasMode, hasDone := false, false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate:
				buf = append(buf, a)
			case ActMode:
				hasMode = true
			case ActDone:
				hasDone = true
			}
		}
		var a Canon
		switch {
		case hasMode && hasDone && seed%3 != 0:
			// take a free cast over stopping on most seeds; the rest decline
			for _, o := range c.D.Options {
				if o.Kind() == ActMode {
					a = o
					break
				}
			}
		case len(buf) > 0:
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		default:
			a = rnd.Pick(c)
		}
		if c.G != nil {
			siskoCount(c, a, t)
		}
		return a
	})
}

func siskoCount(c *PickCtx, a Canon, t *siskoTally) {
	pd := c.G.Pending()
	if pd == nil || pd.ResumeKind != "play" {
		return
	}
	g := c.G.E.G
	src := g.Obj(pd.Source)
	if src == nil || src.Face() == nil || src.Face().Name != "Captain Benjamin Sisko" {
		return
	}
	pl := state.PlayerID(pd.Player)
	bound := siskoOfficers(g, pl)
	t.asks++
	t.officers[bound]++
	offered := map[state.ObjID]bool{}
	for _, o := range pd.Options {
		offered[o.Obj] = true
	}
	for _, id := range g.Zone(state.ZExile, pl) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Owner != pl {
			continue
		}
		f := o.Face()
		if f.IsLand() {
			t.landSeen++
			continue
		}
		if int(f.Cmc()) > bound && !offered[id] {
			t.cut++
		}
	}
	opt, ok := c.G.GorgeOption(a)
	if ok && opt.Kind == "mode" {
		t.picks++
		if len(pd.Options) > 1 {
			t.multiPick++
		}
	} else {
		t.declines++
	}
}

func siskoSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sisko")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range siskoDeck {
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
	return reg, set, decks
}

// siskoRun plays seeds 1..n and returns the divergent games (the first is
// logged).
func siskoRun(t *testing.T, n uint64, tally *siskoTally, stopOnDiv bool) (div int) {
	t.Helper()
	reg, set, decks := siskoSetup(t)
	tbl, _, _, err := set.Matchup(siskoDeck, siskoDeck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sisko-a", "sisko-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, siskoPolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 40})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if div == 1 {
				t.Logf("seed %d diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if stopOnDiv {
				return div
			}
		}
	}
	return div
}

func TestSiskoLockstep(t *testing.T) {
	t.Parallel()
	tally := &siskoTally{officers: map[int]int{}}
	if div := siskoRun(t, 400, tally, false); div != 0 {
		t.Errorf("%d divergent games", div)
	}
	t.Logf("%+v", *tally)
	if tally.asks < 20 {
		t.Errorf("Sisko free-cast asks %d, want at least 20", tally.asks)
	}
	if tally.picks < 10 || tally.declines < 5 {
		t.Errorf("casts %d, declines %d, want at least 10 and 5", tally.picks, tally.declines)
	}
	if len(tally.officers) < 3 {
		t.Errorf("asks at %d distinct Officer counts %v, want at least 3", len(tally.officers), tally.officers)
	}
	if tally.cut < 10 {
		t.Errorf("exiled nonland cards above the bound and withheld: %d, want at least 10", tally.cut)
	}
	if tally.landSeen < 10 {
		t.Errorf("exiled lands seen at an ask: %d, want at least 10", tally.landSeen)
	}
	if tally.multiPick < 5 {
		t.Errorf("casts with more than one castable card left: %d, want at least 5", tally.multiPick)
	}
}

// TestSiskoMutation makes the kernel ignore the mana-value bound (the
// population filter) and requires a divergence.
func TestSiskoMutation(t *testing.T) {
	dirac.DebugFreeFilterIgnored = true
	defer func() { dirac.DebugFreeFilterIgnored = false }()
	if div := siskoRun(t, 400, &siskoTally{officers: map[int]int{}}, true); div == 0 {
		t.Errorf("mutation: no divergence in 400 games")
	}
}
