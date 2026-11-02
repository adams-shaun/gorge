package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cast-battlefield lockstep: Steer Clear (2 damage,
// 4 if a Mount was controlled as it was cast) and Faerie Fencing (-X/-X, an
// extra -3/-3 if a Faerie was controlled as it was cast) over Mounts and
// Faeries, with instant removal on both sides so a Mount or Faerie can die
// between cast and resolution: the frozen count (CR 601.2a) must then differ
// from the live battlefield. Gorge's answer at every decision is the
// expected one.
var kcastbfDecks = [2][]cardset.Entry{
	{ // A: W/U/B, the carriers over Mounts and Faeries: Faerie Fencing's frozen
		// count reads the Faeries (Miscreants and flash Brazen Borrowers, so
		// one can enter between cast and resolution) and Steer Clear reads the
		// Mounts (B's Glare kills an attacking one in response)
		{Name: "Faerie Fencing", Count: 8}, {Name: "Brazen Borrower", Count: 6},
		{Name: "Faerie Miscreant", Count: 2},
		{Name: "Steer Clear", Count: 4}, {Name: "Bridled Bighorn", Count: 4}, {Name: "Alacrian Jaguar", Count: 4},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
	},
	{ // B: the responder: Immolating Glare is B's only spell (destroy target
		// ATTACKING creature, uncashable at sorcery speed, so it is held for
		// the combat windows); the Oxes are fat blockers for A's carriers to
		// target, so a frozen/live difference shows in the projection (a 4/4
		// survives 2 damage or -X but dies to 4 damage or -X-3)
		{Name: "Immolating Glare", Count: 16},
		{Name: "Bulwark Ox", Count: 8},
		{Name: "Plains", Count: 16},
	},
}

func kcastbfSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("castbf")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kcastbfDecks {
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

// kcastbfRun plays seeds 1..seeds with the random policy, reporting
// divergences as test errors when report is set (the mutation run expects
// one and stays quiet).
func kcastbfRun(t *testing.T, seeds uint64, report bool, pol func(seed uint64) Policy) (divs, stalls int) {
	t.Helper()
	reg, set, decks := kcastbfSetup(t)
	tbl, _, _, err := set.Matchup(kcastbfDecks[0], kcastbfDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcastbf-a", "kcastbf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
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

// kcastbfRespondPolicy is the random policy except that it forces the frozen
// count and the live battlefield apart, the case TestCastbfMutation diverges
// on: with a non-empty stack the opponent of the top object casts its first
// cast option (B's Immolating Glare kills an attacking Mount in response),
// and the top object's controller responds to its own spell by flashing in
// Brazen Borrower (a Faerie ENTERS between the cast and the resolution, so
// Faerie Fencing's frozen 0 reads live 1: a difference no fizzle can hide);
// with an empty stack Brazen Borrower is never cast, so one is held for the
// response window. Deterministic: the random policy's own stream and the
// canonical option order decide every other pick.
func kcastbfRespondPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	borrower := func(c *PickCtx, a Canon) bool {
		if a.Kind() != ActCast || c.G == nil || int(a.Src()) >= len(c.G.id.obj) {
			return false
		}
		ob := c.G.E.G.Obj(c.G.id.obj[a.Src()])
		return ob != nil && ob.Face() != nil && ob.Face().Name == "Brazen Borrower"
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || c.G == nil {
			return rnd.Pick(c)
		}
		stk := c.G.E.G.Zone(state.ZStack, 0)
		if len(stk) == 0 {
			// hold the flash Faerie for the response window
			for i := 0; i < len(c.D.Options); i++ {
				a := rnd.Pick(c)
				if !borrower(c, a) {
					return a
				}
			}
			return rnd.Pick(c)
		}
		top := c.G.E.G.Obj(stk[len(stk)-1])
		if top == nil || top.Face() == nil {
			return rnd.Pick(c)
		}
		if top.Controller != state.PlayerID(c.D.Player) {
			// the responder casts its first cast option
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					return a
				}
			}
			return rnd.Pick(c)
		}
		// the caster responds to its own spell by flashing in a Faerie
		for _, a := range c.D.Options {
			if borrower(c, a) {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

func TestCastbfLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.CastBFStatsReset()
	dirac.DebugCastBFStats = true
	defer func() { dirac.DebugCastBFStats = false }()
	divs, stalls := kcastbfRun(t, 256, true, RandomPolicy)
	frozenGE1, frozen0, differ := dirac.CastBFStats()
	t.Logf("256 seeds: %d divergences, %d stalls; ExCastBattlefield reads: %d with a frozen count >= 1, %d with a frozen count of 0, %d whose frozen and live counts differ",
		divs, stalls, frozenGE1, frozen0, differ)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if divs != 0 {
		t.Errorf("%d divergences", divs)
	}
	if frozenGE1 < 30 {
		t.Errorf("only %d reads with a frozen count >= 1", frozenGE1)
	}
	if frozen0 < 30 {
		t.Errorf("only %d reads with a frozen count of 0", frozen0)
	}
}

// TestCastbfMutation: reading the live battlefield at resolution instead of
// the frozen count (a Mount or Faerie dying in response changes the answer)
// must make the lockstep diverge.
func TestCastbfMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCastBFLive = true
	defer func() { dirac.DebugCastBFLive = false }()
	if divs, _ := kcastbfRun(t, 64, false, kcastbfRespondPolicy); divs == 0 {
		t.Fatal("DebugCastBFLive: no divergence in 64 seeds")
	}
}
