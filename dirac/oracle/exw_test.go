package oracle

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The exiled-with source link lockstep (gorge exiledBySourceAt):
// W/B decks over the generated exw pool. The Princess Takes Flight records
// the creature its chapter I exiles and returns "the exiled card" in chapter
// III through the exile sweep (Defined$ ExiledWith); Veteran Survivor records
// the graveyard cards it exiles and counts them in its +3/+3 static
// (ExiledWith$Amount). Cloudshift flickers a source (its reverse link goes
// stale, the forward list is cleared by the re-entry), Journey to Nowhere
// and Murder put other cards into exile, Stitcher's Supplier feeds the
// graveyards.
//
// Seven deck pairs cover every card the exiled-with lowering accepts: the
// leaves-trigger readers (Oblivion Ring, Faceless Butcher, Icy Prison,
// Underworld Sentinel, Trove Warden, Watcher for Tomorrow, Worldgorger
// Dragon, Realm Razer, Day of the Dragons) and the sacrifice-cost, upkeep and
// dies readers (Cold Storage, Synod Sanctum, Endless Sands, Bag of Holding,
// Safe Haven, Helvault, Promise of Tomorrow, Purgatory: two sources of one
// card exile one dying card, the second finds it gone, CR 400.7).
// Prowling Geistcatcher is refused by name (dirac/support/refused), Wormfang
// Newt/Turtle by kgen's exwCheck.
// gorge writes the reverse scalar only for a card whose text names
// ExiledWithSource, so Oblivion Ring, Icy Prison, the Princess and Veteran
// Survivor are forward-list readers and the rest read both.

// kexwPairs are the deck pairs, each played over its own seeds.
type kexwPair struct {
	name  string
	seeds uint64 // 0: the whole run's seeds
	decks [2][]cardset.Entry
}

var kexwPairs = []kexwPair{
	{name: "princess-survivor", decks: [2][]cardset.Entry{
		{ // A
			{Name: "The Princess Takes Flight", Count: 2}, {Name: "Veteran Survivor", Count: 4}, {Name: "Cloudshift", Count: 4},
			{Name: "Murder", Count: 2}, {Name: "Stitcher's Supplier", Count: 3}, {Name: "Journey to Nowhere", Count: 1},
			{Name: "Savannah Lions", Count: 2}, {Name: "Barony Vampire", Count: 1}, {Name: "Serra Angel", Count: 1},
			{Name: "Suntail Hawk", Count: 2}, {Name: "Child of Night", Count: 2},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
		},
		{ // B
			{Name: "The Princess Takes Flight", Count: 3}, {Name: "Veteran Survivor", Count: 4}, {Name: "Cloudshift", Count: 4},
			{Name: "Murder", Count: 1}, {Name: "Stitcher's Supplier", Count: 3}, {Name: "Journey to Nowhere", Count: 1},
			{Name: "Savannah Lions", Count: 3}, {Name: "Barony Vampire", Count: 1}, {Name: "Serra Angel", Count: 1},
			{Name: "Suntail Hawk", Count: 1}, {Name: "Child of Night", Count: 2},
			{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 7},
		},
	}},
	{name: "wb-leavers", decks: [2][]cardset.Entry{
		{
			{Name: "Oblivion Ring", Count: 1}, {Name: "Promise of Tomorrow", Count: 2}, {Name: "Faceless Butcher", Count: 2}, {Name: "Barony Vampire", Count: 2},
			{Name: "Underworld Sentinel", Count: 1}, {Name: "Trove Warden", Count: 2}, {Name: "Child of Night", Count: 2},
			{Name: "Murder", Count: 3}, {Name: "Stitcher's Supplier", Count: 2}, {Name: "Journey to Nowhere", Count: 1},
			{Name: "Savannah Lions", Count: 3}, {Name: "Suntail Hawk", Count: 2}, {Name: "Child of Night", Count: 2},
			{Name: "Barony Vampire", Count: 1}, {Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
		},
		{
			{Name: "Promise of Tomorrow", Count: 2}, {Name: "Faceless Butcher", Count: 2}, {Name: "Barony Vampire", Count: 1},
			{Name: "Underworld Sentinel", Count: 2}, {Name: "Trove Warden", Count: 1}, {Name: "Child of Night", Count: 1},
			{Name: "Murder", Count: 3}, {Name: "Stitcher's Supplier", Count: 3}, {Name: "Journey to Nowhere", Count: 2},
			{Name: "Savannah Lions", Count: 3}, {Name: "Suntail Hawk", Count: 2}, {Name: "Child of Night", Count: 3},
			{Name: "Serra Angel", Count: 1}, {Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
		},
	}},
	{name: "sac-cost-artifacts", decks: [2][]cardset.Entry{
		{
			{Name: "Cold Storage", Count: 5}, {Name: "Bag of Holding", Count: 1}, {Name: "Helvault", Count: 2},
			{Name: "Synod Sanctum", Count: 2}, {Name: "Safe Haven", Count: 2}, {Name: "Endless Sands", Count: 2},
			{Name: "Murder", Count: 2}, {Name: "Savannah Lions", Count: 2}, {Name: "Suntail Hawk", Count: 2},
			{Name: "Child of Night", Count: 3}, {Name: "Serra Angel", Count: 1}, {Name: "Barony Vampire", Count: 1},
			{Name: "Ghost Vacuum", Count: 3},
			{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "Cold Storage", Count: 4}, {Name: "Bag of Holding", Count: 1}, {Name: "Helvault", Count: 1},
			{Name: "Synod Sanctum", Count: 1}, {Name: "Safe Haven", Count: 1}, {Name: "Endless Sands", Count: 1},
			{Name: "Murder", Count: 2}, {Name: "Stitcher's Supplier", Count: 2}, {Name: "Savannah Lions", Count: 3},
			{Name: "Suntail Hawk", Count: 2}, {Name: "Child of Night", Count: 3}, {Name: "Barony Vampire", Count: 2},
			{Name: "Ghost Vacuum", Count: 3},
			{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
		},
	}},
	{name: "uw-hideaway-flicker", decks: [2][]cardset.Entry{
		{
			{Name: "Icy Prison", Count: 5}, {Name: "Watcher for Tomorrow", Count: 4}, {Name: "Day of the Dragons", Count: 1},
			{Name: "Cloudshift", Count: 5}, {Name: "Journey to Nowhere", Count: 2}, {Name: "Savannah Lions", Count: 4},
			{Name: "Suntail Hawk", Count: 3}, {Name: "Oblivion Ring", Count: 2}, {Name: "Island", Count: 7}, {Name: "Plains", Count: 7},
		},
		{
			{Name: "Icy Prison", Count: 4}, {Name: "Watcher for Tomorrow", Count: 3}, {Name: "Day of the Dragons", Count: 2},
			{Name: "Cloudshift", Count: 4}, {Name: "Journey to Nowhere", Count: 2}, {Name: "Savannah Lions", Count: 4},
			{Name: "Suntail Hawk", Count: 3}, {Name: "Serra Angel", Count: 2}, {Name: "Oblivion Ring", Count: 2}, {Name: "Island", Count: 7}, {Name: "Plains", Count: 7},
		},
	}},
	{name: "rwg-mass-exile", decks: [2][]cardset.Entry{
		{
			{Name: "Worldgorger Dragon", Count: 3}, {Name: "Realm Razer", Count: 2}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 3}, {Name: "Suntail Hawk", Count: 3},
			{Name: "Journey to Nowhere", Count: 3}, {Name: "Oblivion Ring", Count: 3}, {Name: "Cloudshift", Count: 2},
			{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3}, {Name: "Plains", Count: 5},
		},
		{
			{Name: "Worldgorger Dragon", Count: 2}, {Name: "Realm Razer", Count: 3}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Suntail Hawk", Count: 2},
			{Name: "Journey to Nowhere", Count: 3}, {Name: "Oblivion Ring", Count: 3}, {Name: "Cloudshift", Count: 2},
			{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3}, {Name: "Plains", Count: 5},
		},
	}},
	{name: "omenpath-northampton", decks: [2][]cardset.Entry{
		{ // Omenpath's search-to-exile then AtRandom$ pick, Northampton's mandatory GainControl$ pick
			{Name: "Omenpath Journey", Count: 4}, {Name: "Northampton Farm", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 2}, {Name: "Suntail Hawk", Count: 2},
			{Name: "Forest", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
		},
		{
			{Name: "Omenpath Journey", Count: 3}, {Name: "Northampton Farm", Count: 5}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Hill Giant", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Suntail Hawk", Count: 2},
			{Name: "Forest", Count: 5}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
		},
	}},
	{name: "purgatory-two-sources", decks: [2][]cardset.Entry{
		{
			{Name: "Purgatory", Count: 4}, {Name: "Murder", Count: 3}, {Name: "Stitcher's Supplier", Count: 3},
			{Name: "Child of Night", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Barony Vampire", Count: 3},
			{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 11},
		},
		{
			{Name: "Purgatory", Count: 3}, {Name: "Murder", Count: 3}, {Name: "Stitcher's Supplier", Count: 3},
			{Name: "Child of Night", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Barony Vampire", Count: 3},
			{Name: "Plains", Count: 11}, {Name: "Swamp", Count: 11},
		},
	}},
	{name: "storm-souls-sweep", decks: [2][]cardset.Entry{
		{ // a graveyard sweep's per-entrant StaticEffect$ (CR 603.6b)
			{Name: "Storm of Souls", Count: 4}, {Name: "Stitcher's Supplier", Count: 4},
			{Name: "Savannah Lions", Count: 4}, {Name: "Suntail Hawk", Count: 4},
			{Name: "Child of Night", Count: 4},
			{Name: "Plains", Count: 14}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "Storm of Souls", Count: 4}, {Name: "Stitcher's Supplier", Count: 4},
			{Name: "Savannah Lions", Count: 4}, {Name: "Suntail Hawk", Count: 4},
			{Name: "Child of Night", Count: 4},
			{Name: "Plains", Count: 14}, {Name: "Swamp", Count: 6},
		},
	}},
}

// kexwTotals is the coverage summed over the pairs; ByCard maps a reading
// card to {links it recorded, reads of it that matched}.
type kexwTotals struct {
	dirac.ExwStats
	ByCard map[string][2]int
	Storm  kexwStormObs
}

// kexwStormObs counts, on the gorge side, Storm of Souls resolutions (the
// spell exiles itself off the stack) and the cards moved from a graveyard
// onto the battlefield. It observes only the pairs whose decks hold Storm
// (kexwHasStorm), where only Storm's sweep moves a card that way.
type kexwStormObs struct {
	Resolved, Entrants int
}

// kexwHasStorm reports whether either deck of pair holds Storm of Souls.
func kexwHasStorm(pair kexwPair) bool {
	for _, d := range pair.decks {
		for _, e := range d {
			if e.Name == "Storm of Souls" {
				return true
			}
		}
	}
	return false
}

// kexwObserve wraps p: it returns p's pick unchanged and first scans the
// gorge event log grown since the last decision. The scan position is local
// to the wrapper, so each game (one wrapper) starts at the head of its log.
func kexwObserve(p Policy, obs *kexwStormObs) Policy {
	evAt := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			evs := c.G.E.L.Events
			for j := evAt; j < len(evs); j++ {
				e := &evs[j]
				if e.Kind != events.MoveZone {
					continue
				}
				switch {
				case e.From == state.ZStack && e.To == state.ZExile:
					if o := g.Obj(e.Obj); o != nil && o.Ability == nil && o.Face() != nil && o.Face().Name == "Storm of Souls" {
						obs.Resolved++
					}
				case e.From == state.ZGraveyard && e.To == state.ZBattlefield:
					obs.Entrants++
				}
			}
			evAt = len(evs)
		}
		return p.Pick(c)
	})
}

// kexwRun plays seeds 1..seeds of every pair and returns whether a game
// diverged, the kernel capacity stall count and the link's coverage counters.
func kexwRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, tot kexwTotals) {
	t.Helper()
	return kexwRunPairs(t, kexwPairs, seeds, report)
}

// kexwRunPairs is kexwRun over the given deck pairs.
func kexwRunPairs(t *testing.T, pairs []kexwPair, seeds uint64, report bool) (diverged bool, stalls int, tot kexwTotals) {
	t.Helper()
	reg, set := kexwPool(t)
	tot.ByCard = map[string][2]int{}
	for _, pair := range pairs {
		n := seeds
		if pair.seeds != 0 && pair.seeds < n {
			n = pair.seeds
		}
		div, st := kexwPlayPair(t, reg, set, pair, 1, n, report, &tot)
		stalls += st
		if div {
			diverged = true
			if !report {
				return
			}
		}
	}
	return
}

// kexwPool loads the corpus and the generated exw set.
func kexwPool(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exw")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kexwPlayPair plays seeds lo..hi of one pair, stopping at its first
// divergence, and adds the link's counters into tot unless a game diverged
// without report. It sets DebugExwStats, a process global the kernel writes
// unsynchronised, so two calls never play side by side in one process.
func kexwPlayPair(t *testing.T, reg *cards.Registry, set cardset.Set, pair kexwPair, lo, hi uint64, report bool, tot *kexwTotals) (diverged bool, stalls int) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range pair.decks {
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
			t.Fatalf("%s deck %d: %d cards", pair.name, s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(pair.decks[0], pair.decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st dirac.ExwStats
	dirac.DebugExwStats = &st
	defer func() { dirac.DebugExwStats = nil }()
	for seed := lo; seed <= hi; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kexw-a", "kexw-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var pol Policy = RandomPolicy(seed)
		if kexwHasStorm(pair) {
			pol = kexwObserve(pol, &tot.Storm)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("%s seed %d: %s", pair.name, seed, res.Stall)
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 400
				if lo < 0 {
					lo = 0
				}
				t.Logf("%s seed %d last actions %v", pair.name, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", pair.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			break
		}
	}
	if diverged && !report {
		return
	}
	tot.Recorded += st.Recorded
	tot.Reads += st.Reads
	tot.Matched += st.Matched
	tot.Rejected += st.Rejected
	tot.AfterLeft += st.AfterLeft
	tot.FwdCleared += st.FwdCleared
	tot.RevCleared += st.RevCleared
	tot.CountPos += st.CountPos
	for i := range tbl.Cards {
		if i < len(st.RecBy) && (st.RecBy[i] != 0 || st.MatchBy[i] != 0) {
			v := tot.ByCard[tbl.Cards[i].Name]
			tot.ByCard[tbl.Cards[i].Name] = [2]int{v[0] + st.RecBy[i], v[1] + st.MatchBy[i]}
		}
	}
	return
}

// kexwTally is one isolated chunk's (or the summed run's) result.
type kexwTally struct {
	Divs, Stalls int
	Tot          kexwTotals
}

// kexwRunIsolated is kexwRunPairs with report: each pair's seeds play in
// chunks of chunk seeds as parallel subtests "seeds/<pair>_<lo>-<hi>", each
// alone in a child process (DebugExwStats is the process's), and the parent
// sums the tallies the children log. In a child it plays its one chunk and
// returns ok false: the caller skips its whole-run checks.
func kexwRunIsolated(t *testing.T, pairs []kexwPair, seeds, chunk uint64) (diverged bool, stalls int, tot kexwTotals, ok bool) {
	t.Helper()
	var sum kexwTally
	sum.Tot.ByCard = map[string][2]int{}
	var mu sync.Mutex
	t.Run("seeds", func(t *testing.T) {
		for _, pair := range pairs {
			n := seeds
			if pair.seeds != 0 && pair.seeds < n {
				n = pair.seeds
			}
			for lo := uint64(1); lo <= n; lo += chunk {
				hi := min(lo+chunk-1, n)
				t.Run(fmt.Sprintf("%s_%d-%d", pair.name, lo, hi), func(t *testing.T) {
					t.Parallel()
					var part kexwTally
					if isolateTally(t, &part) {
						mu.Lock()
						addTally(&sum, &part)
						mu.Unlock()
						return
					}
					part.Tot.ByCard = map[string][2]int{}
					reg, set := kexwPool(t)
					div, st := kexwPlayPair(t, reg, set, pair, lo, hi, true, &part.Tot)
					part.Divs, part.Stalls = b2i(div), st
					logTally(t, &part)
				})
			}
		}
	})
	return sum.Divs > 0, sum.Stalls, sum.Tot, !isolatedChild()
}

func TestExwLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip here, before the chunks (each would
	// skip alone and leave this test's totals empty).
	corpusDirOrSkip(t)
	t.Parallel()
	div, stalls, tot, ok := kexwRunIsolated(t, kexwPairs, 256, kexwChunk)
	if !ok || t.Failed() {
		return
	}
	st := tot.ExwStats
	t.Logf("%d games per pair, exw {Recorded:%d Reads:%d Matched:%d Rejected:%d AfterLeft:%d FwdCleared:%d RevCleared:%d CountPos:%d}, %d divergences, %d stalls",
		256, st.Recorded, st.Reads, st.Matched, st.Rejected, st.AfterLeft, st.FwdCleared, st.RevCleared, st.CountPos, b2i(div), stalls)
	t.Logf("by reading card {links recorded, reads matched}: %v", tot.ByCard)
	t.Logf("Storm of Souls: %d resolutions, %d graveyard sweep entrants", tot.Storm.Resolved, tot.Storm.Entrants)
	if tot.Storm.Resolved == 0 || tot.Storm.Entrants == 0 {
		t.Errorf("Storm of Souls: %d resolutions, %d entrants; the graveyard rider was not exercised", tot.Storm.Resolved, tot.Storm.Entrants)
	}
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{tot.Recorded, 60, "links recorded"},
		{tot.Matched, 20, "ExiledWithSource reads that matched"},
		{tot.Rejected, 10, "reads that rejected a card in exile"},
		{tot.AfterLeft, 5, "matched reads after the source left the battlefield"},
		{tot.FwdCleared, 3, "forward bits cleared by a re-entry"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
	// every reading card the lowering accepts must have been read at least once
	for _, name := range kexwReaders {
		if tot.ByCard[name][1] == 0 {
			t.Errorf("%s: no matched read in the run (links recorded %d)", name, tot.ByCard[name][0])
		}
	}
}

// kexwChunk is the seeds one isolated subtest of TestExwLockstep plays.
const kexwChunk = 128

// kexwReaders are the cards that read the link and lower (the pool's
// ExiledWith readers; the other pool cards only exile, kill or fill).
var kexwReaders = []string{
	"The Princess Takes Flight", "Veteran Survivor",
	"Oblivion Ring", "Faceless Butcher", "Icy Prison", "Underworld Sentinel",
	"Trove Warden", "Cold Storage", "Synod Sanctum", "Endless Sands", "Bag of Holding",
	"Safe Haven", "Helvault", "Watcher for Tomorrow", "Worldgorger Dragon",
	"Realm Razer", "Day of the Dragons", "Promise of Tomorrow",
	"Omenpath Journey", "Northampton Farm", "Ghost Vacuum", "Purgatory",
}

// TestExwMutation: each reader arm the lockstep proves is load-bearing. A
// reader that ignores the source's ver (a flickered source still finds the
// cards its old incarnation exiled) and one that drops the resolving
// trigger's fire-ver arm (a leaves trigger finds nothing) each make the
// lockstep diverge. (Mutations of a "source left, one ver on" arm, a
// stack-entry rebind and the forward-versus-reverse recording split were tried
// and do not diverge in this pool, so that code is not in the kernel.)
func TestExwMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		name string
		flag *bool
	}{
		{"DebugExwIgnoreVer", &dirac.DebugExwIgnoreVer},
		{"DebugExwNoFire", &dirac.DebugExwNoFire},
	} {
		t.Run(m.name, func(t *testing.T) {
			*m.flag = true
			defer func() { *m.flag = false }()
			if diverged, _, _ := kexwRun(t, 64, false); !diverged {
				t.Fatalf("%s: no divergence in 64 seeds", m.name)
			}
		})
	}
}

// The static readers of the link (CR 406.6, 607.2a): Keen-Eyed
// Curator's +4/+4 once four card types are among the cards it exiled, Dawnhand
// Dissident's creature permission (three counters removed from among your
// creatures; its Blight costs put the -1/-1 counters there) and Maralen's
// once-a-turn free cast of a spell with mana value up to your Elves and Faeries
// from among the cards it exiled this turn. They live in their own pairs: the
// matched-read assertion of TestExwLockstep is per reader of kexwPairs.
var kexwStaticPairs = []kexwPair{
	{name: "keen-eyed", decks: [2][]cardset.Entry{
		{
			{Name: "Keen-Eyed Curator", Count: 4}, {Name: "Stitcher's Supplier", Count: 2}, {Name: "Murder", Count: 2},
			{Name: "Giant Growth", Count: 3}, {Name: "Battlegrowth", Count: 2}, {Name: "Cold Storage", Count: 2},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Child of Night", Count: 2},
			{Name: "Forest", Count: 12}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "Keen-Eyed Curator", Count: 3}, {Name: "Stitcher's Supplier", Count: 2}, {Name: "Murder", Count: 3},
			{Name: "Giant Growth", Count: 2}, {Name: "Battlegrowth", Count: 3}, {Name: "Cold Storage", Count: 1},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Child of Night", Count: 2},
			{Name: "Forest", Count: 11}, {Name: "Swamp", Count: 7},
		},
	}},
	{name: "dawnhand", decks: [2][]cardset.Entry{
		{
			{Name: "Dawnhand Dissident", Count: 4}, {Name: "Battlegrowth", Count: 3}, {Name: "Giant Growth", Count: 2},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Stitcher's Supplier", Count: 4}, {Name: "Murder", Count: 2},
			{Name: "Child of Night", Count: 3}, {Name: "Barony Vampire", Count: 2}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 7},
		},
		{
			{Name: "Dawnhand Dissident", Count: 3}, {Name: "Battlegrowth", Count: 2}, {Name: "Giant Growth", Count: 3},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Stitcher's Supplier", Count: 3}, {Name: "Murder", Count: 3},
			{Name: "Child of Night", Count: 3}, {Name: "Barony Vampire", Count: 2}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 7},
		},
	}},
	{name: "maralen", decks: [2][]cardset.Entry{
		{
			{Name: "Maralen, Fae Ascendant", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Faerie Miscreant", Count: 4},
			{Name: "Dawnhand Dissident", Count: 2}, {Name: "Murder", Count: 2}, {Name: "Giant Growth", Count: 2},
			{Name: "Battlegrowth", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Child of Night", Count: 2},
			{Name: "Island", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 5},
		},
		{
			{Name: "Maralen, Fae Ascendant", Count: 3}, {Name: "Llanowar Elves", Count: 4}, {Name: "Faerie Miscreant", Count: 3},
			{Name: "Dawnhand Dissident", Count: 3}, {Name: "Murder", Count: 3}, {Name: "Giant Growth", Count: 2},
			{Name: "Battlegrowth", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Stitcher's Supplier", Count: 2},
			{Name: "Island", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 5},
		},
	}},
	// the Dig reader beside Maralen: an enchantment that exiles the top card of
	// its controller's library and lets them play it (Pick Up the Pace;
	// reverse-only link, the text names ExiledWithSource). Theater of Horrors
	// reads the same way but stays refused on its mixed player/planeswalker
	// target.
	{name: "dig-enchantment", decks: [2][]cardset.Entry{
		{
			{Name: "Pick Up the Pace", Count: 5}, {Name: "Raging Goblin", Count: 7},
			{Name: "Lightning Bolt", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Child of Night", Count: 3},
			{Name: "Murder", Count: 2}, {Name: "Barony Vampire", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
		},
		{
			{Name: "Pick Up the Pace", Count: 5}, {Name: "Raging Goblin", Count: 7},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Child of Night", Count: 2},
			{Name: "Murder", Count: 3}, {Name: "Barony Vampire", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
		},
	}},
	// the two cards the flag lifted beyond the test's three: Hedonist's Trove
	// (a ChangeZoneAll of the opponent's graveyard; two statics, a land one and
	// a once-a-turn spell one) and Rona (a historic exile and a Dig)
	{name: "trove-rona", decks: [2][]cardset.Entry{
		{
			{Name: "Hedonist's Trove", Count: 3}, {Name: "Rona, Disciple of Gix", Count: 4}, {Name: "Stitcher's Supplier", Count: 3},
			{Name: "Murder", Count: 3}, {Name: "Cold Storage", Count: 2}, {Name: "Child of Night", Count: 3},
			{Name: "Barony Vampire", Count: 2}, {Name: "Faerie Miscreant", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Hedonist's Trove", Count: 2}, {Name: "Rona, Disciple of Gix", Count: 4}, {Name: "Stitcher's Supplier", Count: 4},
			{Name: "Murder", Count: 3}, {Name: "Cold Storage", Count: 3}, {Name: "Child of Night", Count: 2},
			{Name: "Barony Vampire", Count: 2}, {Name: "Faerie Miscreant", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 10},
		},
	}},
}

// kexwStaticReaders are the static readers each of which must have matched
// the link at least once in kexwStaticPairs.
var kexwStaticReaders = []string{"Keen-Eyed Curator", "Dawnhand Dissident", "Maralen, Fae Ascendant", "Pick Up the Pace", "Hedonist's Trove", "Rona, Disciple of Gix"}

func TestExwStaticsLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip here, before the chunks (each would
	// skip alone and leave this test's totals empty).
	corpusDirOrSkip(t)
	t.Parallel()
	const seeds = 192
	div, stalls, tot, ok := kexwRunIsolated(t, kexwStaticPairs, seeds, kexwChunk)
	if !ok || t.Failed() {
		return
	}
	st := tot.ExwStats
	t.Logf("%d games per pair, exw {Recorded:%d Reads:%d Matched:%d Rejected:%d AfterLeft:%d}, %d divergences, %d stalls",
		seeds, st.Recorded, st.Reads, st.Matched, st.Rejected, st.AfterLeft, b2i(div), stalls)
	t.Logf("by reading card {links recorded, reads matched}: %v", tot.ByCard)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, name := range kexwStaticReaders {
		if tot.ByCard[name][1] == 0 {
			t.Errorf("%s: no matched read in the run (links recorded %d)", name, tot.ByCard[name][0])
		}
	}
}

// TestExwStaticsMutation: a static that reads no source (DebugExwStaticNoSrc)
// finds none of the cards its host exiled, so Keen-Eyed's pump, Dawnhand's
// permission and Maralen's free cast all vanish and the lockstep must diverge.
func TestExwStaticsMutation(t *testing.T) {
	dirac.DebugExwStaticNoSrc = true
	defer func() { dirac.DebugExwStaticNoSrc = false }()
	if diverged, _, _ := kexwRunPairs(t, kexwStaticPairs, 64, false); !diverged {
		t.Fatal("DebugExwStaticNoSrc: no divergence in 64 seeds")
	}
}
