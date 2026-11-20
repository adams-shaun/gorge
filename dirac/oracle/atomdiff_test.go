package oracle

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// atomSeed is one seed set's outcome: its stats and the first divergence.
type atomSeed struct {
	once  sync.Once
	st    *AtomStats
	fails []string
	games int
	// done is set when the set ran to its end: a set an environment gap
	// skipped (no BLB deck pool, no cmd5 table) leaves it false
	done bool
	// skipped is set when the first caller's t.Skip ended the Do body: a
	// sync.Once marks itself done on that Goexit, so every later caller
	// would see an empty set (and fail on it) instead of skipping too
	skipped bool
}

// load runs body once; a caller that finds the set skipped skips as well.
func (a *atomSeed) load(t *testing.T, body func()) *atomSeed {
	a.once.Do(func() {
		defer func() {
			if t.Skipped() && !a.done {
				a.skipped = true
			}
		}()
		body()
	})
	if a.skipped {
		t.Skip("seed set skipped in this environment (no .cards, deck pool or cmd5 table)")
	}
	return a
}

// atomPairs is the pairs each seed set plays and atomMaxDecisions the cap on a
// game: the check costs registry size times live objects at every decision, so
// the seeds are cut (never the atoms) to keep each test inside its minute on a
// loaded box: eight pairs a set took 95 s (slices) and 79 s (synth) there with
// 76 filter atoms, and the vocabulary is now 200+.
const (
	atomPairs        = 2
	atomMaxDecisions = 400
)

var atomSeeds = map[string]*atomSeed{"slices": {}, "synth": {}, "pod": {}}

// atomPlay plays one lockstep game under AtomCheck and folds its outcome in.
func (a *atomSeed) play(name string, m *Matchup, seed uint64, k Side, cfg Config) {
	st := NewAtomStats()
	st.Seed = seed
	rc := RunConfig{Config: cfg, MaxDecisions: atomMaxDecisions, MaxTurns: 60, Check: AtomCheck(st)}
	res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
	CloseSide(k)
	a.games++
	a.st.Add(st)
	if d := res.Div; d != nil && d.What == "atom" {
		a.fails = append(a.fails, name+": decision "+strconv.Itoa(d.At)+": "+d.Field+": "+d.Detail)
	}
}

func atomSlices(t *testing.T) *atomSeed {
	a := atomSeeds["slices"]
	return a.load(t, func() {
		a.st = NewAtomStats()
		for _, set := range []string{"fdn", "blb"} {
			var c *Corpus
			var pool []PoolDeck
			if set == "fdn" {
				c, pool = testCorpus(t)
			} else {
				c, pool = testPoolCorpus(t, set)
			}
			for _, surface := range []bool{true, false} {
				cfg := Config{GorgeAutopaySurface: surface}
				for _, p := range Pairings(pool, atomPairs, 11) {
					m, tbl, err := SetMatchup(rawSet(set), c, p.A, p.B, 0)
					if err != nil {
						continue
					}
					k, err := SetSide(rawSet(set), tbl, cfg)(m, p.Seed)
					if err != nil {
						continue
					}
					a.play(set+" "+p.A+" vs "+p.B, m, p.Seed, k, cfg)
				}
			}
		}
		a.done = true
	})
}

func atomSynth(t *testing.T) *atomSeed {
	a := atomSeeds["synth"]
	return a.load(t, func() {
		a.st = NewAtomStats()
		for _, name := range []string{"std-blb", "std-otj", "std-mkm"} {
			c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
			set := cardsettest.Synth(t, name)
			pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
			if err != nil {
				t.Fatal(err)
			}
			c.AddDecks(files)
			cfg := Config{GorgeAutopaySurface: true}
			for i := 0; i < atomPairs; i++ {
				p := PairingAt(pool, i, 1)
				m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
				if err != nil {
					continue
				}
				k, err := SetSide(set, tbl, cfg)(m, p.Seed)
				if err != nil {
					t.Fatal(err)
				}
				a.play(name+" "+p.A+" vs "+p.B, m, p.Seed, k, cfg)
			}
		}
		a.done = true
	})
}

func atomPod(t *testing.T) *atomSeed {
	a := atomSeeds["pod"]
	return a.load(t, func() {
		a.st = NewAtomStats()
		c, names := cmd5MiniCorpus(t, 40)
		set, err := cardset.ByName("cmd5")
		if err != nil {
			t.Skipf("no cmd5 table (make dirac-tables): %v", err)
		}
		set = cardset.AllowUnverified(set)
		pool := make([]PoolDeck, len(names))
		for i, n := range names {
			pool[i] = PoolDeck{Name: n}
		}
		p := PodAt(pool, 0, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		a.play("pod", m, p.Seed, k, cfg)
		a.done = true
	})
}

func atomReport(t *testing.T, a *atomSeed) {
	t.Helper()
	for _, f := range a.fails {
		t.Error(f)
	}
	if a.games == 0 || a.st.Decisions == 0 {
		t.Fatalf("%d games, %d compared decisions", a.games, a.st.Decisions)
	}
	t.Logf("%d games, %d decisions", a.games, a.st.Decisions)
}

// TestAtomDiffSlices: the FDN and BLB slices, the first 8 pairs, both surfaces.
func TestAtomDiffSlices(t *testing.T) { atomReport(t, atomSlices(t)) }

// TestAtomDiffSynth: the std-blb, std-otj and std-mkm synthetic pools, 8 pairs each.
func TestAtomDiffSynth(t *testing.T) { atomReport(t, atomSynth(t)) }

// TestAtomDiffPod: one 4-seat commander pod.
func TestAtomDiffPod(t *testing.T) { atomReport(t, atomPod(t)) }

// TestAtomDiffCatchesAWrongWord: an atom whose kernel form is the opposite of
// gorge's (tapped against untapped) is named by a divergence.
func TestAtomDiffCatchesAWrongWord(t *testing.T) {
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	set := cardsettest.Synth(t, "std-blb")
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	cfg := Config{GorgeAutopaySurface: true}
	wrong := FilterAtom{Name: "wrong:tapped", GorgeSpec: "Card.tapped", KernelSpec: "Card.untapped", Ctx: AtomBattlefield}
	for i := 0; i < 8; i++ {
		p := PairingAt(pool, i, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		st := NewAtomStats()
		st.Seed = p.Seed
		rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 60, Check: AtomCheckFor(st, []FilterAtom{wrong}, nil)}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			if d.What != "atom" || d.Field != "wrong:tapped" {
				t.Fatalf("diverged elsewhere: %s %s: %s", d.What, d.Field, d.Detail)
			}
			t.Logf("caught: %s", d.Detail)
			return
		}
	}
	t.Fatal("the wrong atom was never named")
}

// atomUnprovenFile pins the atoms no committed seed reads both true and false
// (writer: PINS_RECORD=1). It may only shrink: a later atom that no seed
// exercises needs a seed that does, not a line here.
const atomUnprovenFile = "testdata/atomdiff-unproven.txt"

// TestAtomCoverage lists the atoms the seeds leave unproven.
func TestAtomCoverage(t *testing.T) {
	// the three seed sets play side by side (a set an earlier test already
	// played is reused), so the test stays inside one set's minute
	seeds := map[string]func(*testing.T) *atomSeed{"slices": atomSlices, "synth": atomSynth, "pod": atomPod}
	t.Run("seeds", func(t *testing.T) {
		for _, name := range []string{"slices", "synth", "pod"} {
			f := seeds[name]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				atomReport(t, f(t))
			})
		}
	})
	total := NewAtomStats()
	for _, name := range []string{"slices", "synth", "pod"} {
		a := seeds[name](t)
		if !a.done {
			// the pin is a measurement over all three sets: over a partial
			// run (the precheck has no GORGE_POOLS) it would only mismatch
			t.Skipf("seed set %s did not run to its end in this environment: the unproven pin is not comparable", name)
		}
		total.Add(a.st)
	}
	fa, ca := RegisteredAtoms()
	var unproven []string
	for _, n := range func() []string {
		var ns []string
		for _, a := range fa {
			ns = append(ns, a.Name)
		}
		for _, a := range ca {
			ns = append(ns, a.Name)
		}
		return ns
	}() {
		if total.True[n] == 0 || total.False[n] == 0 {
			unproven = append(unproven, n)
		}
	}
	sort.Strings(unproven)
	got := strings.Join(unproven, "\n") + "\n"
	if os.Getenv("PINS_RECORD") != "" {
		if err := os.WriteFile(atomUnprovenFile, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(filepath.FromSlash(atomUnprovenFile))
	if err != nil {
		t.Fatal(err)
	}
	pinned := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") { // names hold spaces
		pinned[l] = true
	}
	now := map[string]bool{}
	for _, n := range unproven {
		now[n] = true
		if !pinned[n] {
			t.Errorf("atom %s is unproven (true %d, false %d over %d decisions): add a seed that reads it both ways", n, total.True[n], total.False[n], total.Decisions)
		}
	}
	for n := range pinned {
		if !now[n] {
			t.Errorf("atom %s is proven now: delete it from %s (the pin only shrinks)", n, atomUnprovenFile)
		}
	}
	t.Logf("%d atoms, %d unproven", len(fa)+len(ca), len(unproven))
}

// TestAtomRegistryLowers: kgen lowers every registered atom in every zone its
// Ctx names (a word kgen refuses is not an atom of this harness: narrow its
// Ctx or leave it out).
func TestAtomRegistryLowers(t *testing.T) {
	fa, ca := RegisteredAtoms()
	if len(fa) == 0 || len(ca) == 0 {
		t.Fatalf("%d filter atoms, %d count atoms registered", len(fa), len(ca))
	}
	set := cardsettest.Synth(t, "std-blb")
	pool, files, err := SynthPool(synthReg(t), set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
	c.AddDecks(files)
	p := PairingAt(pool, 0, 1)
	_, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := compileAtoms(fa, ca, tbl).err; err != nil {
		t.Error(err)
	}
}
