package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ward—sacrifice lockstep (CR 702.21a): Vein Ripper (Ward—
// Sacrifice a creature) and Ygra, Eater of All (Ward—Sacrifice a Food; its
// static makes every other creature a Food, so the pick's candidates are
// layer-derived) against targeted removal and burn that trigger the ward,
// beside Food makers and creatures to pay with. Bloodbriar ("whenever you
// sacrifice another permanent") is on the paying side: the ward payment is a
// real sacrifice (gorge PaySacrificeCost), so it fires the
// listener. Gorge's answer at every decision is the expected one.
var kwardsacDecks = [2][]cardset.Entry{
	{ // A: B/G
		{Name: "Vein Ripper", Count: 6}, {Name: "Ygra, Eater of All", Count: 6}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Bake into a Pie", Count: 2}, {Name: "Detective's Satchel", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hungry Ghoul", Count: 1}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
	},
	{ // B: R/G, the targeting side, with creatures to pay the ward
		{Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Hill Giant", Count: 1},
		{Name: "Bloodbriar", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}

type kwardsacStats struct {
	games, elections, payments, vrPay, ygraPay, ygraFoodOnly, bloodbriarFire int
}

// kwardsacPolicy counts the ward—sacrifice elections and picks (gorge's
// pending decision when it leads) around a cast-first policy (random on every fourth seed).
func kwardsacPolicy(seed uint64, st *kwardsacStats) Policy {
	rnd := castFirstPolicy(seed) // the carriers are expensive: cast whenever offered
	if seed%4 == 0 {
		rnd = RandomPolicy(seed)
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil {
			return a
		}
		if d.ResumeSA != nil && d.ResumeKind == "unless_pay" && strings.HasPrefix(d.ResumeSA.ParamStr(cards.PKUnlessCost), "Sac<") {
			st.elections++
			// gorge's two-option KModes [pay, decline]: pay seven times in eight
			if c.D.Kind == DModes && len(c.D.Options) == 2 && st.elections%8 != 0 {
				a = c.D.Options[0]
			}
		}
		if len(d.Options) == 0 || d.Options[0].Kind != "ward_sac" {
			return a
		}
		st.payments++
		ygra := strings.Contains(d.ResumeSA.ParamStr(cards.PKUnlessCost), "Food")
		if ygra {
			st.ygraPay++
		} else {
			st.vrPay++
		}
		for _, o := range d.Options {
			if c.G.InstRef(o.Obj) != a.Src() {
				continue
			}
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
				if bb := c.G.E.G.Obj(id); id != o.Obj && bb != nil && bb.Face() != nil && bb.Face().Name == "Bloodbriar" {
					st.bloodbriarFire++ // the payer's Bloodbriar sees the pick sacrificed
					break
				}
			}
			if ob := c.G.E.G.Obj(o.Obj); ygra && ob != nil && ob.Face() != nil && ob.Face().IsCreature() {
				st.ygraFoodOnly++ // a creature, a Food only through Ygra's static
			}
		}
		return a
	})
}

func kwardsacRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *kwardsacStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wardsac")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwardsacDecks {
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
	tbl, _, _, err := set.Matchup(kwardsacDecks[0], kwardsacDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &kwardsacStats{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwardsac-a", "kwardsac-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kwardsacPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestWardsacLockstep(t *testing.T) {
	t.Parallel()
	divs, stalls, st := kwardsacRun(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls; %d ward—sacrifice elections, %d payments (%d Vein Ripper, %d Ygra, %d of them a creature Food only through Ygra), %d declined or unpayable; %d payments with a Bloodbriar in play (its Sacrificed trigger fires)",
		st.games, divs, stalls, st.elections, st.payments, st.vrPay, st.ygraPay, st.ygraFoodOnly, st.elections-st.payments, st.bloodbriarFire)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.payments < 30 {
		t.Errorf("%d ward—sacrifice payments, want >= 30", st.payments)
	}
	if st.elections-st.payments < 30 {
		t.Errorf("%d declined or unpayable ward—sacrifice counters, want >= 30", st.elections-st.payments)
	}
	if st.bloodbriarFire < 5 {
		t.Errorf("%d ward—sacrifice payments with a Bloodbriar in play, want >= 5", st.bloodbriarFire)
	}
	if st.ygraFoodOnly < 5 {
		t.Errorf("%d Ygra payments with a creature Food only through Ygra, want >= 5", st.ygraFoodOnly)
	}
}

// TestWardsacMutation: a pick that offers every permanent the payer
// controls must make the lockstep diverge.
func TestWardsacMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugWardSacAnyPerm = true
	defer func() { dirac.DebugWardSacAnyPerm = false }()
	if divs, _, _ := kwardsacRun(t, 64, false); divs == 0 {
		t.Fatal("DebugWardSacAnyPerm: no divergence in 64 seeds")
	}
}
