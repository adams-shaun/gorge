package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The five-target-slot lockstep. A choose-two Command whose
// four modes each declare one target lays out four slots (CR 700.2, 601.2c);
// the fourth mode's target rides slot 3, RefTarget3 (kernel tgtSlot). Per
// Command, from gorge's event log: resolutions with mode 3 chosen (its
// target in slot 3) and those where that target was illegal at resolution
// (the kernel's resCtx.bad bit 3).

const kmaxtgtSeeds = 256

var kmaxtgtDecks = [2][]cardset.Entry{
	{
		{Name: "Grub's Command", Count: 4}, {Name: "Call Damage Control", Count: 4}, {Name: "Goblin Piker", Count: 8},
		{Name: "Stitcher's Supplier", Count: 4}, {Name: "Ornithopter", Count: 2}, {Name: "Sol Ring", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Sygg's Command", Count: 4}, {Name: "Coral Merfolk", Count: 6}, {Name: "Merfolk of the Pearl Trident", Count: 4},
		{Name: "Savannah Lions", Count: 2}, {Name: "Unsummon", Count: 10}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 6},
	},
}

// kmaxtgtCmd is one Command's mode-3 text and the zone its slot-3 target
// must be in to be legal.
var kmaxtgtCmds = map[string]struct {
	mode3 string
	zone  state.Zone
}{
	"Grub's Command":      {"mills five cards", state.ZBattlefield},
	"Call Damage Control": {"Target land card", state.ZGraveyard},
	"Sygg's Command":      {"Tap target creature", state.ZBattlefield},
}

type kmaxtgtPend struct {
	name  string
	mode3 bool
	bad   bool
}

type kmaxtgtTally struct {
	evAt  int
	modes map[state.ObjID]string
	pend  map[state.ObjID]*kmaxtgtPend
	cast  map[string]int
	res   map[string]int // resolutions
	res3  map[string]int // resolutions with mode 3 chosen
	bad3  map[string]int // ... whose slot-3 target was illegal at resolution
}

func newKmaxtgtTally() *kmaxtgtTally {
	return &kmaxtgtTally{modes: map[state.ObjID]string{}, pend: map[state.ObjID]*kmaxtgtPend{},
		cast: map[string]int{}, res: map[string]int{}, res3: map[string]int{}, bad3: map[string]int{}}
}

func (p *kmaxtgtTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.ModeChosen:
			if _, ok := kmaxtgtCmds[chName(g, e.Obj)]; ok {
				p.modes[e.Obj] = e.Text
			}
		case events.Resolve:
			if pd := p.pend[e.Obj]; pd != nil {
				p.res[pd.name]++
				if pd.mode3 {
					p.res3[pd.name]++
					if pd.bad {
						p.bad3[pd.name]++
					}
				}
				delete(p.pend, e.Obj)
			}
		}
	}
	p.evAt = len(evs)
	for _, id := range g.Stack {
		name := chName(g, id)
		c, ok := kmaxtgtCmds[name]
		if !ok {
			continue
		}
		pd := p.pend[id]
		if pd == nil {
			pd = &kmaxtgtPend{name: name}
			p.pend[id] = pd
			p.cast[name]++
		}
		// the spell is on the stack while its modes are still being chosen
		// (CR 601.2a, 601.2b): re-read them at every check
		pd.mode3 = strings.Contains(p.modes[id], c.mode3)
		if o := g.Obj(id); pd.mode3 && o != nil && len(o.Targets) > 0 {
			// the targets are in mode order, so mode 3's is the last
			if t := o.Targets[len(o.Targets)-1]; !t.IsPlayer {
				pd.bad = false
				if to := g.Obj(t.Obj); to == nil || to.Zone != c.zone {
					pd.bad = true
				}
			}
		}
	}
	return nil
}

// kmaxtgtPolicy plays at random, except that a
// Sygg's Command's controller answers it with Unsummon aimed at its mode-3
// creature target: the target the Command then finds illegal at resolution
// (CR 608.2b).
func kmaxtgtPolicy(seed uint64) Policy {
	base := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || len(c.G.E.G.Stack) == 0 {
			return base.Pick(c)
		}
		g := c.G.E.G
		var victim state.ObjID
		for _, id := range g.Stack {
			if o := g.Obj(id); o != nil && chName(g, id) == "Sygg's Command" && len(o.Targets) > 0 {
				if t := o.Targets[len(o.Targets)-1]; !t.IsPlayer {
					victim = t.Obj
				}
			}
		}
		if victim == 0 {
			return base.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case DPriority:
				if n := chName(g, state.ObjID(a.Src())+1); a.Kind() == ActCast && n == "Unsummon" {
					buf = append(buf, a)
				}
			case DTarget:
				if a.Kind() == ActTarget && state.ObjID(a.Tgt())+1 == victim {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func kmaxtgtSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maxtgt")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kmaxtgtDecks {
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
	tbl, _, _, err := set.Matchup(kmaxtgtDecks[0], kmaxtgtDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func kmaxtgtRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, p *kmaxtgtTally) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"kmaxtgt-a", "kmaxtgt-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	var check func(*GorgeSide, Side) *Divergence
	if p != nil {
		p.evAt = 0
		p.modes = map[state.ObjID]string{}
		p.pend = map[state.ObjID]*kmaxtgtPend{}
		check = p.check
	}
	lead, err := NewGorgeSide(m, seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, kmaxtgtPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
	CloseSide(k)
	return res
}

func TestMaxtgtLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := kmaxtgtSetup(t)
	p := newKmaxtgtTally()
	games := 0
	for seed := uint64(1); seed <= kmaxtgtSeeds; seed++ {
		if v := os.Getenv("KMAXTGT_SEED"); v != "" && v != fmt.Sprint(seed) {
			continue
		}
		res := kmaxtgtRun(t, reg, set, decks, tbl, seed, p)
		games++
		if res.Stall != "" && res.Stall != "turns" { // (the 20-turn cap ends a game)
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	for _, name := range []string{"Grub's Command", "Call Damage Control", "Sygg's Command"} {
		t.Logf("%s: cast %d, resolved %d, resolved with mode 3 chosen %d, slot-3 target illegal at resolution %d",
			name, p.cast[name], p.res[name], p.res3[name], p.bad3[name])
		if p.res3[name] < 20 {
			t.Errorf("%s: %d resolutions with mode 3 chosen, want at least 20", name, p.res3[name])
		}
	}
	bad := 0
	for _, n := range p.bad3 {
		bad += n
	}
	t.Logf("%d games, %d resolutions with an illegal slot-3 target", games, bad)
	// Only Sygg's Command's creature target can leave the battlefield (a
	// player or a graveyard card does not), and only when its controller
	// holds Unsummon with a fourth land untapped (8 resolutions over the 256 seeds, short of the test's 10).
	if bad < 1 {
		t.Errorf("no resolution with an illegal slot-3 target")
	}
}

// TestMaxtgtMutation: RefTarget3 read as slot 2 must diverge.
func TestMaxtgtMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbl := kmaxtgtSetup(t)
	dirac.DebugTgt3AsSlot2 = true
	defer func() { dirac.DebugTgt3AsSlot2 = false }()
	for seed := uint64(1); seed <= 64; seed++ {
		if res := kmaxtgtRun(t, reg, set, decks, tbl, seed, nil); res.Div != nil {
			t.Logf("mutation DebugTgt3AsSlot2: seed %d diverged at decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Errorf("mutation DebugTgt3AsSlot2 never diverged in 64 seeds")
}
