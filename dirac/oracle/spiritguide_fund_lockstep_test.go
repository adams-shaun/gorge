package oracle

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Spirit Guide lockstep (CR 605.1a, 605.2a): Simian and Elvish Spirit
// Guide's hand-zone mana ability is a hand source of gorge's manual payment
// windows (pay.ManaSourceIDs lists the hand after the battlefield) and of its
// untapped-source gates (hasUntappedManaSource). The guide deck holds few
// lands, so its casts meet the windows short of battlefield mana: the other
// deck's unless counters (Mana Leak, Force Spike, Spell Pierce) and unless
// triggers (Rhystic Study, Esper Sentinel), its ward creature (Waterfall
// Aerialist) that the guide deck's burn targets, and the guide deck's own
// announced X (Banefire). The expected answer at every decision is gorge's.
var kspiritGuideDecks = [2][]cardset.Entry{
	{
		{Name: "Simian Spirit Guide", Count: 6}, {Name: "Elvish Spirit Guide", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Simic Cluestone", Count: 4}, {Name: "Signpost Scarecrow", Count: 4},
		{Name: "Banefire", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Mana Leak", Count: 3}, {Name: "Force Spike", Count: 3}, {Name: "Spell Pierce", Count: 3},
		{Name: "Counterspell", Count: 2}, {Name: "Rhystic Study", Count: 2}, {Name: "Esper Sentinel", Count: 3},
		{Name: "Waterfall Aerialist", Count: 4}, {Name: "Wind Drake", Count: 3},
		{Name: "Island", Count: 13}, {Name: "Plains", Count: 4},
	},
}

func TestSpiritGuideFundLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "spiritguide", kspiritGuideDecks, 400, "KSPIRITGUIDE_SEEDS")
}

// TestSpiritGuideFundLockstepMutation proves the pool pays with the guides: with
// every hand guide left out of the kernel's windows and gates, some seed
// diverges.
func TestSpiritGuideFundLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoHandMana = true
	defer func() { dirac.DebugNoHandMana = false }()
	if lockstepDiverges(t, "spiritguide", kspiritGuideDecks, 80) {
		return
	}
	t.Fatal("no seed diverged with the hand guides left out: the pool does not pay with them")
}

// TestSpiritGuideFundSearchMutation proves the pool funds a filter's fee by
// exiling a guide: with the hand guides left out of the prefix search's
// funding pass, some seed diverges.
func TestSpiritGuideFundSearchMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoHandFund = true
	defer func() { dirac.DebugNoHandFund = false }()
	if lockstepDiverges(t, "spiritguide", kspiritGuideDecks, 200) {
		return
	}
	t.Fatal("no seed diverged with the hand guides left out of the search: the pool never funds a fee with one")
}

// spiritGuidePodNames is deck d of kspiritGuideDecks as a card-name list.
func spiritGuidePodNames(d int) []string {
	var names []string
	for _, e := range kspiritGuideDecks[d] {
		for i := 0; i < e.Count; i++ {
			names = append(names, e.Name)
		}
	}
	return names
}

// TestSpiritGuideFundFourSeatPodLockstep plays the guide decks in four-seat
// pods (guide, counters, guide, counters): the funding pass and the hand mana
// windows past two seats. Four seats lead a commander pod; the expected
// answer at every decision is gorge's.
func TestSpiritGuideFundFourSeatPodLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(300)
	if v := os.Getenv("KSPIRITGUIDE4_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	a, b := spiritGuidePodNames(0), spiritGuidePodNames(1)
	decks := [4][]string{a, b, a, b}
	reg := synthReg(t)
	set, err := cardset.RawByName("spiritguide")
	if err != nil {
		t.Fatal(err)
	}
	var clean int
	var bad []string
	seedChunks(t, 1, seeds, 25, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		var lists [][]cardset.Entry
		for _, d := range decks {
			es := make([]cardset.Entry, len(d))
			for i, n := range d {
				es[i] = cardset.Entry{Name: n, Count: 1}
			}
			lists = append(lists, es)
		}
		tbl, _, err := set.MatchupN(lists)
		if err != nil {
			t.Fatal(err)
		}
		for seed := lo; seed <= hi; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
				MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions, rc.MaxTurns = 3000, 30
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
			CloseSide(k)
			mu.Lock()
			switch {
			case res.Div != nil:
				bad = append(bad, fmt.Sprintf("seed %d: decision %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff))
			case res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix):
				bad = append(bad, fmt.Sprintf("seed %d: kernel stopped: %s", seed, res.Stall))
			default:
				clean++
			}
			mu.Unlock()
		}
	})
	t.Logf("four-seat spiritguide pods: %d seeds, %d clean", seeds, clean)
	if len(bad) > 0 {
		slices.Sort(bad)
		t.Errorf("%d of %d pods diverge:\n%s", len(bad), seeds, strings.Join(bad, "\n"))
	}
}

// TestSpiritGuideFundKeyCountMutation proves the pool reaches a pool holding
// a Signpost's tagged unit beside a guide's plain one: keyed by the per-colour
// bit instead of the count, the kernel's visited set dedups a node gorge's
// does not, and some seed diverges.
func TestSpiritGuideFundKeyCountMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugArtKeyBit = true
	defer func() { dirac.DebugArtKeyBit = false }()
	if lockstepDiverges(t, "spiritguide", kspiritGuideDecks, 400) {
		return
	}
	t.Fatal("no seed diverged with the tagged units keyed by their colour bit: the pool never mixes tagged and plain mana")
}

// TestSpiritGuideUnlessWindowFilterMutation proves the pool pays a Mana Leak
// through a filter source (Signpost Scarecrow's "{2}: Add one mana of any
// color"): gorge's unless window carries no pending cast, so no fee-only
// stall prunes the filter; applied there, some seed diverges.
func TestSpiritGuideUnlessWindowFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugWinFeeStallsAlways = true
	defer func() { dirac.DebugWinFeeStallsAlways = false }()
	if lockstepDiverges(t, "spiritguide", kspiritGuideDecks, 400) {
		return
	}
	t.Fatal("no seed diverged with the fee-only stall applied to the unless window: the pool never pays through a filter")
}
