package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The not-enchanted-by lockstep: Due Diligence's enters
// trigger targets a creature you control other than enchanted creature, and
// Sporogenic Infection's has a target player sacrifice a creature other than
// enchanted creature (kop.FNotAttachedBySrc, gorge's !attachedBy). Pacifism
// and Unholy Strength are plain Auras beside them. Gorge's answer at every
// decision is the expected one.
var knotenchDecks = [2][]cardset.Entry{
	{ // A: W/B, the three Auras' worth of carriers
		{Name: "Due Diligence", Count: 4}, {Name: "Sporogenic Infection", Count: 4}, {Name: "Unholy Strength", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 4},
	},
	{ // B: a generic creature deck
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Pacifism", Count: 2}, {Name: "Due Diligence", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
	},
}

var knotenchCarrier = map[string]bool{"Due Diligence": true, "Sporogenic Infection": true}

type knotenchStats struct {
	games, triggers, live int // carrier triggers resolved; of them, with the Aura attached and >= 2 creatures on that side
	byCard                map[string]int
}

func knotenchSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("notench")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range knotenchDecks {
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
	return reg, set, decks
}

// knotenchProbe follows each carrier trigger on gorge's stack and, when it
// leaves, counts it live if at its last sample the Aura was attached and the
// side the filter reads (Due Diligence: its controller; Sporogenic
// Infection: the enchanted creature's controller) had >= 2 creatures, so
// the exclusion narrowed a real choice.
type knotenchProbe struct {
	stats *knotenchStats
	open  map[state.ObjID]knotenchItem
}

type knotenchItem struct {
	name string
	live bool
}

func knotenchCreatures(g *state.Game, p state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().IsCreature() {
			n++
		}
	}
	return n
}

func (p *knotenchProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	on := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || o.Ability == nil {
			continue
		}
		src := g.Obj(o.Source)
		if src == nil || src.Face() == nil || !knotenchCarrier[src.Face().Name] {
			continue
		}
		on[id] = true
		it := knotenchItem{name: src.Face().Name}
		if src.Zone == state.ZBattlefield && src.AttachedTo != 0 {
			side := o.Controller
			if it.name == "Sporogenic Infection" {
				if h := g.Obj(src.AttachedTo); h != nil {
					side = h.Controller
				}
			}
			it.live = knotenchCreatures(g, side) >= 2
		}
		p.open[id] = it
	}
	for id, it := range p.open {
		if !on[id] {
			p.done(it)
			delete(p.open, id)
		}
	}
	return nil
}

func (p *knotenchProbe) done(it knotenchItem) {
	p.stats.triggers++
	if it.live {
		p.stats.live++
		p.stats.byCard[it.name]++
	}
}

// knotenchRun plays seeds 1..seeds with the probe, reporting divergences as
// test errors when report is set (the mutation run expects one and stays
// quiet).
func knotenchRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *knotenchStats) {
	t.Helper()
	reg, set, decks := knotenchSetup(t)
	tbl, _, _, err := set.Matchup(knotenchDecks[0], knotenchDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &knotenchStats{byCard: map[string]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"knotench-a", "knotench-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &knotenchProbe{stats: st, open: map[state.ObjID]knotenchItem{}}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: pr.check})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !report {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestNotenchLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := knotenchRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d carrier triggers resolved, %d live (Aura attached, >= 2 creatures on the read side; minimum 40): %v",
		st.games, divs, stalls, st.triggers, st.live, st.byCard)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.live < 40 {
		t.Errorf("%d live !EnchantedBy resolutions, want >= 40", st.live)
	}
}

// TestNotenchMutation: FNotAttachedBySrc read as the positive form must make
// the lockstep diverge.
func TestNotenchMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNotAttachedPositive = true
	defer func() { dirac.DebugNotAttachedPositive = false }()
	if divs, _, _ := knotenchRun(t, 64, false); divs == 0 {
		t.Fatal("DebugNotAttachedPositive: no divergence in 64 seeds")
	}
}
