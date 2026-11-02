package oracle

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The count-value-modifier lockstep: the three carriers of the
// new value grammar lowered by kgen --
//
//   - Colossus of the Blood Age: Remembered$Amount/Plus.1 (draw remembered+1);
//   - Dark Matter Manipulator: Times.2 after a DivideEvenlyDown.7;
//   - Outlaw Stitcher: Twice after a Minus.1 (X may be 0 or -2 -> no counters).
//
// Hit the Mother Lode's Remembered$CardManaCost/NMinus.10 and constant cmcLT
// lower, but its Discover free cast plus DB$ Token tail is refused by kgen:
// the kernel defers the free cast past the tail (kernel-discover-freecast-order),
// so it is deliberately not in this pool. NMinus and the constant cmc compare
// are pinned by the kgen lowering tests (dirac/kgen/value_refusals_test.go and the
// Discover-tail test in dirac/kgen/valuemods_test.go).
//
// Cheap spells move Outlaw Stitcher's cast count and self-mill crosses the
// Manipulator's seven-card thresholds.
var kvaluemodsDecks = [2][]cardset.Entry{
	{
		{Name: "Colossus of the Blood Age", Count: 3}, {Name: "Outlaw Stitcher", Count: 3},
		{Name: "Dark Matter Manipulator", Count: 2},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 2}, {Name: "Unsummon", Count: 1},
		{Name: "Giant Growth", Count: 1}, {Name: "Thought Scour", Count: 2}, {Name: "Mind Sculpt", Count: 1},
		{Name: "Doom Blade", Count: 2},
		{Name: "Craw Wurm", Count: 1}, {Name: "Shivan Dragon", Count: 1}, {Name: "Mahamoti Djinn", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Hill Giant", Count: 1},
		{Name: "Mountain", Count: 5}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 2},
		{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Dark Matter Manipulator", Count: 3}, {Name: "Colossus of the Blood Age", Count: 2},
		{Name: "Outlaw Stitcher", Count: 2},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 2}, {Name: "Mind Rot", Count: 2},
		{Name: "Thought Scour", Count: 1}, {Name: "Tome Scour", Count: 1}, {Name: "Mind Sculpt", Count: 1},
		{Name: "Doom Blade", Count: 3},
		{Name: "Force of Nature", Count: 1}, {Name: "Shivan Dragon", Count: 1}, {Name: "Ancient Stone Idol", Count: 1},
		{Name: "Gray Ogre", Count: 1}, {Name: "Serra Angel", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 3},
		{Name: "Island", Count: 2}, {Name: "Plains", Count: 1},
	},
}

// kvmCounts is the pool's exercise census over a run.
type kvmCounts struct {
	colossus, colossusDiscard  int // death triggers, those that discarded >= 1
	manipulator                int // boards at +2 or more power
	stitcher, stitcherX0       int // ETBs, those whose 2/2 token got X=0
	stitcherPos, stitcherNeg   int // ETBs whose X was >0 / negative (clamped)
	tokenWrong                 int // tokens whose counters are not max(X, 0)
	games, divergences, stalls int
}

// kvmGame tracks one game's object identities (IDs restart every game).
type kvmGame struct {
	colossusBoard, colossusDead, colossusDiscarded map[state.ObjID]bool
	dmm, stitcher, token                           map[state.ObjID]bool
}

func newKVMGame() *kvmGame {
	return &kvmGame{
		colossusBoard:     map[state.ObjID]bool{},
		colossusDead:      map[state.ObjID]bool{},
		colossusDiscarded: map[state.ObjID]bool{},
		dmm:               map[state.ObjID]bool{},
		stitcher:          map[state.ObjID]bool{},
		token:             map[state.ObjID]bool{},
	}
}

// observe counts card events from gorge's state at every decision. It runs
// right after the policy's pick, so the discard pick (a) is the one about to
// resolve; state from the previous action is already visible.
func (o *kvmGame) observe(c *PickCtx, a Canon, n *kvmCounts) {
	if c.G == nil {
		return
	}
	g := c.G.E.G
	if d := c.G.Pending(); d != nil {
		if ob := g.Obj(d.Source); ob != nil && ob.Face() != nil &&
			ob.Zone == state.ZGraveyard && ob.Face().Name == "Colossus of the Blood Age" {
			o.colossusDead[d.Source] = true
			if opt, ok := c.G.GorgeOption(a); ok && opt.Kind == "discard" && !o.colossusDiscarded[d.Source] {
				o.colossusDiscarded[d.Source] = true
				n.colossusDiscard++
			}
		}
	}
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Face() == nil {
			continue
		}
		switch ob.Face().Name {
		case "Colossus of the Blood Age":
			switch ob.Zone {
			case state.ZBattlefield:
				o.colossusBoard[ob.ID] = true
			case state.ZGraveyard:
				if o.colossusBoard[ob.ID] && !o.colossusDead[ob.ID] {
					o.colossusDead[ob.ID] = true
					n.colossus++
				}
			}
		case "Dark Matter Manipulator":
			if ob.Zone == state.ZBattlefield && c.G.E.Power(ob.ID) >= 3 && !o.dmm[ob.ID] {
				o.dmm[ob.ID] = true
				n.manipulator++
			}
		case "Outlaw Stitcher":
			if ob.Zone == state.ZBattlefield && !o.stitcher[ob.ID] {
				o.stitcher[ob.ID] = true
				n.stitcher++
			}
		case "Zombie Rogue Token":
			if ob.Zone != state.ZBattlefield || o.token[ob.ID] {
				continue
			}
			o.token[ob.ID] = true
			// Outlaw Stitcher's X is twice (spells its controller has cast
			// this turn - 1). The ETB trigger reads gorge's per-seat turn
			// watcher, which already counts the Stitcher spell itself, so
			// classify X from that watcher rather than from the counter
			// count: 0 spells is the X=-2 clamp path, 1 is X=0 and >=2 is
			// X>0. The placed counters must be max(X, 0).
			x := 2 * (int(g.Players[int(ob.Controller)].SpellsCastThisTurn) - 1)
			want := x
			if want < 0 {
				want = 0
			}
			if got := int(ob.Counter("P1P1")); got != want {
				n.tokenWrong++
			}
			switch {
			case x < 0:
				n.stitcherNeg++
			case x == 0:
				n.stitcherX0++
			default:
				n.stitcherPos++
			}
		}
	}
}

// kvmRun plays seeds 1..maxSeed and returns the exercise census.
func kvmRun(t *testing.T, maxSeed uint64) kvmCounts {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("valuemods")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kvaluemodsDecks {
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
	tbl, _, _, err := set.Matchup(kvaluemodsDecks[0], kvaluemodsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var n kvmCounts
	for seed := uint64(1); seed <= maxSeed; seed++ {
		m, err := NewMatchup(reg, [2]string{"kvaluemods-a", "kvaluemods-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var base Policy = castFirstPolicy(seed)
		if seed%8 == 0 {
			base = RandomPolicy(seed)
		}
		game := newKVMGame()
		castTurn, casts := -1, 0
		pol := PolicyFunc(func(c *PickCtx) Canon {
			a := base.Pick(c)
			// Stay under the kernel's hard cast ring (castRingN spells
			// pushed per player per turn; more is a loud StallCastRing, not a
			// divergence). A long game with treasured mana can fund a sixth
			// cheap spell; pass instead so the pool measures the value
			// grammar, not that capacity cap. The 0-stall result is therefore
			// policy-bounded: it does not prove the ring is never reached.
			if c.G.Turn() != castTurn {
				castTurn, casts = c.G.Turn(), 0
			}
			if a.Kind() == ActCast {
				if casts >= 5 {
					for _, o := range c.D.Options {
						if o.Kind() == ActPass {
							a = o
							break
						}
					}
				} else {
					casts++
				}
			}
			game.observe(c, a, &n)
			return a
		})
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		n.games++
		if d := res.Div; d != nil {
			n.divergences++
			lo := max(d.At-16, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			n.stalls++
			t.Errorf("seed %d: kernel stall: %s", seed, res.Stall)
		}
	}
	return n
}

func TestValuemodsLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(300)
	if v := os.Getenv("KVALUEMODS_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	n := kvmRun(t, seeds)
	t.Logf("valuemods: %d seeds, %d divergences, %d kernel stalls; Colossus deaths %d (discarded %d), Manipulator boards +2 %d, Stitcher ETBs %d (X=0 %d, X>0 %d, X<0 %d), token counter mismatches %d",
		n.games, n.divergences, n.stalls, n.colossus, n.colossusDiscard, n.manipulator, n.stitcher, n.stitcherX0, n.stitcherPos, n.stitcherNeg, n.tokenWrong)
	if n.divergences != 0 {
		t.Fatalf("%d divergences; want 0", n.divergences)
	}
	if n.stalls != 0 {
		t.Fatalf("%d kernel stalls; want 0", n.stalls)
	}
	if n.colossus < 10 || n.colossusDiscard < 10 {
		t.Errorf("Colossus of the Blood Age: %d death triggers (%d with a discard); want >= 10 and >= 10", n.colossus, n.colossusDiscard)
	}
	if n.manipulator < 10 {
		t.Errorf("Dark Matter Manipulator: %d boards at +2 power; want >= 10", n.manipulator)
	}
	if n.stitcher < 10 || n.stitcherX0 < 3 {
		t.Errorf("Outlaw Stitcher: %d ETBs (%d with X=0); want >= 10 and >= 3", n.stitcher, n.stitcherX0)
	}
	if n.stitcherPos < 1 {
		t.Errorf("Outlaw Stitcher: no ETB with X>0; want >= 1")
	}
	if n.tokenWrong != 0 {
		t.Errorf("Outlaw Stitcher: %d tokens whose counters are not max(X, 0); want 0", n.tokenWrong)
	}
}
