package oracle

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The second-mana-ability lockstep (dirac/cardset/pools/fixtures/manasecond):
//
//   - Baxter Building: "{T}: Add {C}" beside "{4}, {T}: Add four mana in any
//     combination of colors" (Produced$ Combo Any | Amount$ 4). gorge's
//     planner census prices only the free {C} ability, so a play the census
//     cannot pay takes the prefix search, which activates the filter (paying
//     its {4} from four Wastes) and allocates the four mana over the
//     colours the cost needs. Breya, Etherium Shaper costs {W}{U}{B}{R}, so
//     it is payable only through the filter.
//   - Haunted Screen: "{T}: Add {W} or {B}" beside "{T}, Pay 1 life: Add {G},
//     {U}, or {R}". gorge prices the first as its normal alternatives and
//     the second as a last-resort Consequence{Life: 1}, as it does a
//     painland's damage; the kernel's pain source follows. Moss Viper's {G}
//     is payable only through the life ability; Valiant Guard's {W}
//     through the free one.
//
// Each deck holds many identical copies of each mana source, so most
// payments that tap one choose among identical copies: the comparison is
// the source tie-break too — gorge's key-8 witness (pay.CompareSteps over
// the census order) against the kernel's lexLeast — not only the shapes.
//
// The expected answer at every decision is gorge's.
var kmanasecondDecks = [2][]cardset.Entry{
	{
		{Name: "Baxter Building", Count: 14}, {Name: "Breya, Etherium Shaper", Count: 8},
		{Name: "Wastes", Count: 18},
	},
	{
		{Name: "Haunted Screen", Count: 14}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Suntail Hawk", Count: 8}, {Name: "Wastes", Count: 10},
	},
}

// manasecondSeeds is the seeds the lockstep plays, 1..manasecondSeeds, in
// manasecondRanges' contiguous ranges, one test each so every test fits the
// one-minute budget (the single test over the 200 seeds ran 1406 s in CI).
// KMANASECOND_SEEDS=N plays seeds 1..N of them instead: a range wholly above N
// skips, one that straddles N plays up to N.
const (
	manasecondSeeds = 200
	manasecondChunk = 25
)

// manasecondRange plays seeds lo..hi of the second-ability lockstep (clipped
// by KMANASECOND_SEEDS) and fails on every divergence.
func manasecondRange(t *testing.T, lo, hi uint64) {
	t.Parallel()
	if v := os.Getenv("KMANASECOND_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			if lo > n {
				t.Skipf("KMANASECOND_SEEDS=%d: seeds %d-%d not played", n, lo, hi)
			}
			hi = min(hi, n)
		}
	}
	reg, set, gd := lockstepPool(t, "manasecond", kmanasecondDecks)
	clean, casts, div := lockstepSeedsMode(t, reg, set, "manasecond", kmanasecondDecks, gd, lo, hi, lockstepQuiet, nil, nil, castFirstPolicy)
	t.Logf("manasecond: seeds %d-%d, %d clean; %d casts/activations", lo, hi, clean, casts)
	if len(div) > 0 {
		t.Error(divergenceReport("manasecond", hi, div))
	}
}

// TestSecondManaAbilityExercised is the precondition: the fixture plays these
// cards (a table refuses a deck holding a card it did not lower), and each
// intended payment happens.
func TestSecondManaAbilityExercised(t *testing.T) {
	t.Parallel()
	exercised := map[string]int{}
	manasecondRun(t, 24, func(m *Matchup, res *Result) {
		for _, a := range res.Actions {
			if a.Kind() == ActCast {
				if src := a.Src(); src != RefNone {
					exercised[m.InstanceCard(int(src))]++
				}
			}
		}
	})
	for _, name := range []string{"Breya, Etherium Shaper", "Grizzly Bears", "Suntail Hawk"} {
		if exercised[name] == 0 {
			t.Fatalf("no %s cast in 24 seeds: the pool does not exercise the shape (%v)", name, exercised)
		}
	}
}

func TestSecondManaAbilityLockstep001to025(t *testing.T) { manasecondRange(t, 1, 25) }
func TestSecondManaAbilityLockstep026to050(t *testing.T) { manasecondRange(t, 26, 50) }
func TestSecondManaAbilityLockstep051to075(t *testing.T) { manasecondRange(t, 51, 75) }
func TestSecondManaAbilityLockstep076to100(t *testing.T) { manasecondRange(t, 76, 100) }
func TestSecondManaAbilityLockstep101to125(t *testing.T) { manasecondRange(t, 101, 125) }
func TestSecondManaAbilityLockstep126to150(t *testing.T) { manasecondRange(t, 126, 150) }
func TestSecondManaAbilityLockstep151to175(t *testing.T) { manasecondRange(t, 151, 175) }
func TestSecondManaAbilityLockstep176to200(t *testing.T) { manasecondRange(t, 176, 200) }

// TestSecondManaAbilityRangesCoverTheSeeds keeps the ranges above exactly
// 1..manasecondSeeds: nothing trimmed, nothing played twice.
func TestSecondManaAbilityRangesCoverTheSeeds(t *testing.T) {
	src, err := os.ReadFile("mana_second_ability_test.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`manasecondRange\(t, (\d+), (\d+)\)`)
	next := uint64(1)
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		lo, _ := strconv.ParseUint(m[1], 10, 64)
		hi, _ := strconv.ParseUint(m[2], 10, 64)
		if lo != next || hi < lo || hi-lo+1 > manasecondChunk {
			t.Fatalf("range %d-%d: want it to start at %d and span at most %d seeds", lo, hi, next, manasecondChunk)
		}
		next = hi + 1
	}
	if next != manasecondSeeds+1 {
		t.Fatalf("the ranges end at seed %d, want %d", next-1, manasecondSeeds)
	}
}

// manasecondRun plays seeds lockstep games of the fixture, failing on a
// divergence, and hands each clean result to check.
func manasecondRun(t *testing.T, seeds uint64, check func(m *Matchup, res *Result)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manasecond")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kmanasecondDecks {
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
	tbl, _, _, err := set.Matchup(kmanasecondDecks[0], kmanasecondDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"manasecond-a", "manasecond-b"}, gd, 1)
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
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast actions %v (implicates %v)",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions,
				actionStrings(res.Actions[lo:d.At]), d.Implicates(m))
		}
		if check != nil {
			check(m, &res)
		}
	}
}
