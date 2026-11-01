package oracle

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: hand-to-battlefield moves whose filter may
// admit an Aura (CR 303.4f/g) and a counted "any number" ChangeNum$ over
// the generated handaura pool:
//
//   - Kona, Rescue Beastie, Anticausal Vestige and Mind into Matter put a
//     permanent card (an Aura among them) from the hand onto the battlefield;
//   - Origin of the Avengers puts a Hero creature card;
//   - Ghalta, Stampede Tyrant puts any number of creature cards
//     (ChangeNum$ HandX over Count$ValidHand Creature.YouCtrl).
//
// An Aura so put enchants what its controller chooses as it enters: the one
// legal creature unasked, an ask for two or more, and with none it stays in
// the hand. Pacifism, Holy Strength, Claustrophobia and Shardmage's Rescue
// (enchant a creature you control) are the Auras.

var khaDecks = [2][]cardset.Entry{
	{
		{Name: "Kona, Rescue Beastie", Count: 3}, {Name: "Ghalta, Stampede Tyrant", Count: 3}, {Name: "Origin of the Avengers", Count: 2},
		{Name: "Anticausal Vestige", Count: 2}, {Name: "White Tiger, Amulet Keeper", Count: 2}, {Name: "Winter Soldier, Bucky Barnes", Count: 2},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Pacifism", Count: 2}, {Name: "Holy Strength", Count: 2}, {Name: "Shardmage's Rescue", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Mind into Matter", Count: 4}, {Name: "Ghalta, Stampede Tyrant", Count: 2}, {Name: "Kona, Rescue Beastie", Count: 3},
		{Name: "Anticausal Vestige", Count: 2}, {Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 3},
		{Name: "White Tiger, Amulet Keeper", Count: 2}, {Name: "Claustrophobia", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 8},
	},
}

var khaCarriers = []string{"Kona, Rescue Beastie", "Anticausal Vestige", "Mind into Matter", "Origin of the Avengers", "Ghalta, Stampede Tyrant"}

func khaIsAura(ob *state.Object) bool {
	return ob != nil && ob.Face() != nil && slices.Contains(ob.Face().Types, "Aura")
}

// khaPolicy plays eagerPolicy, except that a carrier's pick takes an Aura from
// the hand three times in four. From gorge's pending decision it counts the
// Aura picks, the enchant asks, the entries and the stays, and Ghalta's
// resolutions that move a creature.
func khaPolicy(seed uint64, n khgCounts) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	rnd := rand.New(rand.NewPCG(seed, 0x68616e64)) // "hand"
	var pend state.ObjID                           // the Aura picked, not yet seen entered or stayed
	asked := false
	ghalta := false // the previous decision was a Ghalta pick
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		d := c.G.Pending()
		if d == nil {
			return inner.Pick(c)
		}
		enchant := slices.ContainsFunc(d.Options, func(o decision.Option) bool { return o.Kind == "enchant" })
		if pend != 0 {
			switch ob := c.G.E.G.Obj(pend); {
			case enchant:
				if !asked {
					n["ask"]++
					asked = true
				}
			case ob != nil && ob.Zone == state.ZBattlefield:
				n["entered"]++
				pend, asked = 0, false
			default:
				n["stay"]++
				pend, asked = 0, false
			}
		}
		src := ""
		if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
			src = ob.Face().Name
		}
		a := inner.Pick(c)
		if !slices.Contains(khaCarriers, src) {
			ghalta = false
			return a
		}
		for _, o := range c.D.Options {
			if opt, ok := c.G.GorgeOption(o); ok && c.G.E.G.Obj(opt.Obj) != nil && c.G.E.G.Obj(opt.Obj).Zone == state.ZHand && khaIsAura(c.G.E.G.Obj(opt.Obj)) && rnd.IntN(4) != 0 {
				a = o
				break
			}
		}
		opt, ok := c.G.GorgeOption(a)
		if !ok || opt.Obj == 0 {
			ghalta = false
			return a
		}
		ob := c.G.E.G.Obj(opt.Obj)
		if ob == nil || ob.Zone != state.ZHand {
			ghalta = false
			return a
		}
		if khaIsAura(ob) {
			n["aura_pick"]++
			pend, asked = opt.Obj, false
		}
		if src == "Ghalta, Stampede Tyrant" {
			if !ghalta {
				n["ghalta_etb"]++
			}
			n["ghalta_pick"]++
			ghalta = true
		} else {
			ghalta = false
		}
		return a
	})
}

// khaRun plays seeds 1..maxSeed; it returns the counts, the games played, the
// first divergence ("" if none) and the kernel stalls.
func khaRun(t *testing.T, maxSeed uint64) (n khgCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("handaura")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range khaDecks {
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
	tbl, _, _, err := set.Matchup(khaDecks[0], khaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	n = khgCounts{}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= maxSeed; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kha-a", "kha-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, khaPolicy(seed, n), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v\nref %v\nker %v\n%v %s", seed, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions, d.Diff, d.Detail)
			return n, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return n, games, "", stalls
}

func TestHandauraLockstep(t *testing.T) {
	t.Parallel()

	n, games, div, stalls := khaRun(t, 400)
	t.Logf("%d games; Aura picks %d: entered %d (enchant asks %d), stayed in hand %d; Ghalta resolutions moving a creature %d (%d creatures); stalls %v",
		games, n["aura_pick"], n["entered"], n["ask"], n["stay"], n["ghalta_etb"], n["ghalta_pick"], stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if n["entered"] < 10 || n["ask"] < 3 || n["stay"] < 2 {
		t.Errorf("Auras entered %d, enchant asks %d, stays %d, want at least 10, 3 and 2", n["entered"], n["ask"], n["stay"])
	}
	if n["ghalta_etb"] < 5 {
		t.Errorf("Ghalta resolutions moving a creature %d, want at least 5", n["ghalta_etb"])
	}
}

// TestHandauraMutation: with the Aura entered unattached (the state-based
// actions put it into the graveyard, where gorge has it attached) the
// lockstep must diverge.
func TestHandauraMutation(t *testing.T) {
	dirac.DebugHandAuraNoAttach = true
	defer func() { dirac.DebugHandAuraNoAttach = false }()
	_, games, div, _ := khaRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugHandAuraNoAttach", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
