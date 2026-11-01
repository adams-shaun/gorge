package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 614.1a, 614.9): ReplaceDyingDefined$
// Targeted on a Pump or Fight over the generated exileexmark pool.
//
//   - Gnashing of Teeth (-5/-5), Necrotic Wound (-X/-X) and Bleed Dry
//     (-13/-13): the Pump's target is exiled instead of dying this turn;
//   - Faunsbane Troll (Fight, Sac an attached Aura as the cost): only the
//     fought target is marked, not the Troll (Defined$ Self);
//   - Shock and the fights finish marked creatures, creatures of toughness
//     2-6 survive a small pump or fight, and the mark ends at cleanup.
var kexileexmarkDecks = [2][]cardset.Entry{
	{
		{Name: "Faunsbane Troll", Count: 4}, {Name: "Blanchwood Armor", Count: 4}, {Name: "Gnashing of Teeth", Count: 3},
		{Name: "Necrotic Wound", Count: 2}, {Name: "Bleed Dry", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Trained Armodon", Count: 2}, {Name: "Drudge Skeletons", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Trained Armodon", Count: 3}, {Name: "Colossal Dreadmaw", Count: 3},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Blight Mamba", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
}

// kexmCounts is what gorge's event log showed.
type kexmCounts struct {
	marked   int // creatures a marker spell or ability targeted
	exiled   int // ... that left the battlefield for exile (the replacement)
	died     int // ... that went to the graveyard (an unmarked or earlier death)
	survived int // ... still on the battlefield when the turn ended
}

var kexmSources = map[string]bool{
	"Gnashing of Teeth": true, "Necrotic Wound": true, "Bleed Dry": true, "Faunsbane Troll": true,
}

func kexmScan(g *GorgeSide) kexmCounts {
	var c kexmCounts
	if g == nil {
		return c
	}
	evs := g.E.L.Events
	for i, ev := range evs {
		if ev.Kind != events.TargetsChosen || ev.Amount != 0 {
			continue
		}
		src := g.E.G.Obj(ev.Obj)
		if src == nil || src.Card == nil || !kexmSources[src.Face().Name] {
			continue
		}
		for _, id := range ev.IDs {
			o := g.E.G.Obj(id)
			if o == nil || o.Card == nil || o.IsToken {
				continue
			}
			c.marked++
			res := "survived"
		walk:
			for _, e := range evs[i+1:] {
				switch {
				case e.Kind == events.TurnChange:
					break walk
				case e.Kind == events.MoveZone && e.Obj == id && e.From == state.ZBattlefield:
					if e.To == state.ZExile {
						res = "exiled"
					} else {
						res = "died"
					}
					break walk
				}
			}
			switch res {
			case "exiled":
				c.exiled++
			case "died":
				c.died++
			default:
				c.survived++
			}
		}
	}
	return c
}

// kexmPolicy wraps base so at most one marker spell or ability is put on the
// stack per turn: a second exile-instead mark on one creature is two CR 616.1
// replacements, the kernel's stall (StallReplLeave), not a pump-and-fight case
// this lockstep measures. A canonical card ref is the card's index in the two
// decks laid end to end (deck 0 is 0..39, deck 1 40..79).
func kexmPolicy(seed uint64, base Policy) Policy {
	var names []string
	for _, d := range kexileexmarkDecks {
		for _, e := range d {
			for i := 0; i < e.Count; i++ {
				names = append(names, e.Name)
			}
		}
	}
	var turn int
	var used bool
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.G.Turn() != turn {
			turn, used = c.G.Turn(), false
		}
		marker := func(a Canon) bool {
			k := a.Kind()
			return (k == ActCast || k == ActCastFace || k == ActActivate) && int(a.Src()) < len(names) && kexmSources[names[a.Src()]]
		}
		if used {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if !marker(a) {
					buf = append(buf, a)
				}
			}
			if len(buf) != 0 && len(buf) != len(c.D.Options) {
				d := *c.D
				d.Options = buf
				c = &PickCtx{D: &d, G: c.G, Seq: c.Seq}
			}
		}
		a := base.Pick(c)
		if marker(a) {
			used = true
		}
		return a
	})
}

// kexmRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func kexmRun(t *testing.T, seeds uint64) (tot kexmCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exileexmark")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kexileexmarkDecks {
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
	tbl, _, _, err := set.Matchup(kexileexmarkDecks[0], kexileexmarkDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kexm-a", "kexm-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := kexmPolicy(seed, castFirstPolicy(seed))
		if seed%8 == 0 {
			pol = kexmPolicy(seed, RandomPolicy(seed))
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
			t.Logf("seed %d stalled: %s; last actions %v", seed, res.Stall, actionStrings(res.Actions[max(len(res.Actions)-14, 0):]))
		}
		c := kexmScan(gs)
		tot.marked += c.marked
		tot.exiled += c.exiled
		tot.died += c.died
		tot.survived += c.survived
		if d := res.Div; d != nil {
			lo := max(d.At-60, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestExileexmarkLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := kexmRun(t, 256)
	t.Logf("%d games; marked creatures %d: exiled instead of dying %d, died %d, survived the turn %d; stalls %v",
		games, tot.marked, tot.exiled, tot.died, tot.survived, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.exiled < 15 {
		t.Errorf("marked creatures exiled instead of dying %d, want at least 15", tot.exiled)
	}
	if tot.survived < 5 {
		t.Errorf("marked creatures that survived the turn %d, want at least 5", tot.survived)
	}
}

// TestExileexmarkMutation: with the mark op doing nothing the marked
// creatures die to the graveyard, and the lockstep must diverge within 64
// seeds.
func TestExileexmarkMutation(t *testing.T) {
	dirac.DebugExileExMarkOff = true
	defer func() { dirac.DebugExileExMarkOff = false }()
	_, games, div, _ := kexmRun(t, 64)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugExileExMarkOff", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
