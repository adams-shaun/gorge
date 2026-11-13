package oracle

import (
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The granted-ward lockstep (CR 702.21a-b): Sheltered by Ghosts and
// Hardlight Containment (Auras granting ward {2} / {1}), Combat Research (a
// legendary enchanted creature's ward {1}) and Coppercoat Vanguard (each
// other Human's ward {1}) beside Humans, legends and an artifact to carry
// them, and burn, bounce, Swords to Plowshares and Valorous Stance to target them.
// The expected answer at every decision is gorge's.
var kwardDecks = [2][]cardset.Entry{
	{
		{Name: "Sheltered by Ghosts", Count: 4}, {Name: "Combat Research", Count: 3}, {Name: "Coppercoat Vanguard", Count: 4},
		{Name: "Hardlight Containment", Count: 2}, {Name: "Kitsa, Otterball Elite", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 2},
		{Name: "Thraben Inspector", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Ornithopter", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Unsummon", Count: 3}, {Name: "Swords to Plowshares", Count: 3},
		{Name: "Valorous Stance", Count: 2}, {Name: "Coppercoat Vanguard", Count: 2}, {Name: "Elite Vanguard", Count: 2}, {Name: "Savannah Lions", Count: 2},
		{Name: "Wind Drake", Count: 2}, {Name: "Hill Giant", Count: 1}, {Name: "Pacifism", Count: 2}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 3},
	},
}

func TestWardLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ward", kwardDecks, 1000, "KWARD_SEEDS")
}

// TestWardLockstepMutation proves the pool exercises the granted wards:
// with the kernel's granted triggers dropped, some seed diverges.
func TestWardLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoWardGrant = true
	defer func() { dirac.DebugNoWardGrant = false }()
	if lockstepDiverges(t, "ward", kwardDecks, 40) {
		return
	}
	t.Fatal("no seed diverged with the granted ward triggers dropped: the pool does not exercise them")
}

// lockstepSet plays seeds constructed lockstep games of decks over the
// generated set (seed%4 == 0 on gorge's autopay surface, seed%8 == 0 with
// the random policy, else cast-first) and fails on every divergence; env
// overrides the seed count.
func lockstepSet(t *testing.T, setName string, decks [2][]cardset.Entry, seeds uint64, env string) {
	t.Helper()
	if v := os.Getenv(env); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	reg, set, gd := lockstepPool(t, setName, decks)
	var clean, casts int
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		c, n, _ := lockstepSeeds(t, reg, set, setName, decks, gd, lo, hi, true, nil, nil, castFirstPolicy)
		mu.Lock()
		clean, casts = clean+c, casts+n
		mu.Unlock()
	})
	t.Logf("%s: %d seeds, %d clean; %d casts/activations", setName, seeds, clean, casts)
}

// lockstepChunk is the seeds one parallel subtest of lockstepSet plays
// (seedChunks).
const lockstepChunk = 50

// lockstepSetSerial is lockstepSet's games in one loop, for a test whose
// kernel hooks count in plain (non-atomic) package variables.
func lockstepSetSerial(t *testing.T, setName string, decks [2][]cardset.Entry, seeds uint64, env string) {
	t.Helper()
	if v := os.Getenv(env); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	lockstepRun(t, setName, decks, seeds, true, nil)
}

// lockstepDiverges reports whether any of the first seeds games diverges
// (a mutation test's proof that the pool exercises a mechanism).
func lockstepDiverges(t *testing.T, setName string, decks [2][]cardset.Entry, seeds uint64) bool {
	t.Helper()
	return lockstepRun(t, setName, decks, seeds, false, nil) > 0
}

// lockstepRun plays the games; report fails the test on each divergence
// (else the first stops the run). It returns the divergent games.
func lockstepRun(t *testing.T, setName string, decks [2][]cardset.Entry, seeds uint64, report bool, mutate func(*kop.Table)) int {
	t.Helper()
	return lockstepRunSide(t, setName, decks, seeds, report, mutate, nil)
}

// lockstepRunSide is lockstepRun with wrap, when set, applied to the kernel side.
func lockstepRunSide(t *testing.T, setName string, decks [2][]cardset.Entry, seeds uint64, report bool, mutate func(*kop.Table), wrap func(Side) Side) int {
	t.Helper()
	return lockstepRunPolicy(t, setName, decks, seeds, report, mutate, wrap, castFirstPolicy)
}

// lockstepRunPolicy is lockstepRunSide with the policy of each non-random seed
// chosen by the caller.
func lockstepRunPolicy(t *testing.T, setName string, decks [2][]cardset.Entry, seeds uint64, report bool, mutate func(*kop.Table), wrap func(Side) Side, policy func(uint64) Policy) int {
	t.Helper()
	reg, set, gd := lockstepPool(t, setName, decks)
	clean, casts, div := lockstepSeeds(t, reg, set, setName, decks, gd, 1, seeds, report, mutate, wrap, policy)
	if div > 0 && !report {
		return div
	}
	t.Logf("%s: %d seeds, %d clean; %d casts/activations", setName, seeds, clean, casts)
	return div
}

// lockstepPool loads the corpus and the generated set and builds the two
// gorge decks.
func lockstepPool(t *testing.T, setName string, decks [2][]cardset.Entry) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(setName)
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	return reg, set, gd
}

// lockstepSeeds plays seeds lo..hi of lockstepRunPolicy's games on a table of
// its own and returns the clean games, the casts and activations, and the
// divergent games (without report, it stops at the first).
func lockstepSeeds(t *testing.T, reg *cards.Registry, set cardset.Set, setName string, decks [2][]cardset.Entry, gd [2][]*cards.Card, lo, hi uint64, report bool, mutate func(*kop.Table), wrap func(Side) Side, policy func(uint64) Policy) (clean, casts, div int) {
	t.Helper()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := lo; seed <= hi; seed++ {
		m, err := NewMatchup(reg, [2]string{setName + "-a", setName + "-b"}, gd, 1)
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
		if wrap != nil {
			k = wrap(k)
		}
		pol := policy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		for _, a := range res.Actions {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				casts++
			}
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return clean, casts, div
			}
			from := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
		clean++
	}
	return clean, casts, div
}

// kwardLifeDecks pairs a printed "ward—pay 2 life" (Zul'Ashur, Lich Lord)
// with a granted ward {1} (Combat Research) against burn and bounce: each
// instance triggers (CR 113.2c), the printed one and every granted one.
var kwardLifeDecks = [2][]cardset.Entry{
	{
		{Name: "Zul Ashur, Lich Lord", Count: 8}, {Name: "Combat Research", Count: 8}, {Name: "Thraben Inspector", Count: 2},
		{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 6}, {Name: "Unsummon", Count: 6},
		{Name: "Elite Vanguard", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Island", Count: 6},
	},
}

// TestWardLifeWardLockstep: a printed life ward and the granted wards beside
// it all trigger when the matchup has a granted ward (wardGranted once dropped
// the printed one, then the granted ones).
func TestWardLifeWardLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ward", kwardLifeDecks, 200, "KWARD_SEEDS")
}

// TestWardLifeWardMutation: with the printed life ward dropped some game
// diverges, so the pool does reach a permanent holding both wards.
func TestWardLifeWardMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDropLifeWard = true
	defer func() { dirac.DebugDropLifeWard = false }()
	if lockstepDiverges(t, "ward", kwardLifeDecks, 60) {
		return
	}
	t.Fatal("no seed diverged with the printed life ward dropped: the pool does not reach Zul'Ashur with a granted ward")
}
