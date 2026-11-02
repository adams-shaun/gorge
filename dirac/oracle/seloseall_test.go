package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: a ChangeZone StaticEffect$ whose
// body reads RemoveAllAbilities$ True.
//
//   - Xu-Ifit, Osteoharmonist returns a creature card from the graveyard as
//     a Skeleton with no abilities (a Vampire Nighthawk loses flying,
//     deathtouch and lifelink; an Elvish Visionary has already drawn);
//   - Hellcat, Undying Vigilante returns itself with a +1/+1 counter, no
//     abilities and the haste its own record grants.
//
// Shock empties the battlefield into the graveyards.
var kseloseallDecks = [2][]cardset.Entry{
	{
		{Name: "Xu-Ifit, Osteoharmonist", Count: 4}, {Name: "Vampire Nighthawk", Count: 4}, {Name: "Elvish Visionary", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 6},
		{Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Hellcat, Undying Vigilante", Count: 4}, {Name: "Elvish Visionary", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 6},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 6},
	},
}

type ksleCounts struct {
	xu     int // creature cards returned from a graveyard that were not Hellcat
	hell   int // Hellcat returns from a graveyard
	hasted int // returned Hellcats that attacked the turn they returned
}

func ksleScan(gs *GorgeSide) ksleCounts {
	var c ksleCounts
	if gs == nil {
		return c
	}
	g := gs.E.G
	var back []state.ObjID // Hellcats returned this turn
	for _, ev := range gs.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			back = back[:0]
		case events.MoveZone:
			if ev.From != state.ZGraveyard || ev.To != state.ZBattlefield {
				continue
			}
			if ob := g.Obj(ev.Obj); ob != nil && strings.HasPrefix(ob.Face().Name, "Hellcat") {
				c.hell++
				back = append(back, ev.Obj)
			} else {
				c.xu++
			}
		case events.DeclareAttackers:
			for _, id := range ev.IDs {
				for i, b := range back {
					if b == id {
						c.hasted++
						back = append(back[:i], back[i+1:]...)
						break
					}
				}
			}
		}
	}
	return c
}

func ksleSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("seloseall")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kseloseallDecks {
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
	tbl, _, _, err := set.Matchup(kseloseallDecks[0], kseloseallDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// ksleRun plays seeds 1..seeds over tbl and returns the summed counts, the
// first divergence ("" if none) and the kernel stalls.
func ksleRun(t *testing.T, seeds uint64, tbl *kop.Table) (tot ksleCounts, games int, div string, stalls map[string]int) {
	reg, set, decks, _ := ksleSetup(t)
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kse-a", "kse-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
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
		}
		c := ksleScan(gs)
		tot.xu += c.xu
		tot.hell += c.hell
		tot.hasted += c.hasted
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestSeloseallLockstep(t *testing.T) {
	t.Parallel()
	_, _, _, tbl := ksleSetup(t)
	tot, games, div, stalls := ksleRun(t, 256, tbl)
	t.Logf("%d games; graveyard returns %d (Hellcat %d, attacked the turn it returned %d); stalls %v", games, tot.xu, tot.hell, tot.hasted, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.xu < 20 {
		t.Errorf("Xu-Ifit returns %d, want at least 20", tot.xu)
	}
	if tot.hell < 10 {
		t.Errorf("Hellcat returns %d, want at least 10", tot.hell)
	}
	if tot.hasted < 5 {
		t.Errorf("Hellcat attacks the turn it returned %d, want at least 5", tot.hasted)
	}
}

// TestSeloseallMutation: with LoseAbilities cleared on the pool table's
// StaticEffect override records the lockstep must diverge within 64 seeds.
func TestSeloseallMutation(t *testing.T) {
	t.Parallel()
	_, _, _, tbl := ksleSetup(t)
	mut := *tbl
	mut.Overrides = append([]kop.Override(nil), tbl.Overrides...)
	n := 0
	for i := range mut.Overrides {
		if mut.Overrides[i].LoseAbilities {
			mut.Overrides[i].LoseAbilities = false
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d override records lose abilities, want 2 (Xu-Ifit, Hellcat)", n)
	}
	_, games, div, _ := ksleRun(t, 64, &mut)
	if div == "" {
		t.Fatalf("no divergence in %d games with LoseAbilities cleared", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
