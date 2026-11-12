package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated
// namedother pool, whose carriers read a named<X> filter (gorge wordNamed
// -> sharesName, CR 201.2). Rufus Shinra's attack trigger is gated on "you
// control no creature named Darkstar" (its own token); Dragonstorm
// Forecaster searches for a card named Dragonstorm Globe or Boulderborn
// Dragon (deck A holds the Globe only, so the Boulderborn alternative names
// no object in the matchup and lowers to PredN -1); Gruff Triplets puts
// counters on each creature you control named Gruff_Triplets, its own name
// spelled with '_' (nameArg). Murder and burn kill Darkstar and the
// Triplets so the gate and the death trigger both see each side.

var knamedotherDecks = [2][]cardset.Entry{
	// W/U/B: Rufus, the Forecaster and the Globe it finds, with Murder
	// for the Triplets
	{
		{Name: "Rufus Shinra", Count: 4}, {Name: "Dragonstorm Forecaster", Count: 4},
		{Name: "Dragonstorm Globe", Count: 3}, {Name: "Murder", Count: 3},
		{Name: "Savannah Lions", Count: 2}, {Name: "Vampire Neonate", Count: 2},
		{Name: "Coral Merfolk", Count: 2}, {Name: "Ornithopter", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 6},
	},
	// G/R: the Triplets, ramp and burn for Darkstar
	{
		{Name: "Gruff Triplets", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 6},
	},
}

func knamedotherSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("namedother")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// knamedotherSeen counts the events the minimums read, across games.
type knamedotherSeen struct {
	rufusCreate, rufusPresent int // Rufus attacks with no Darkstar / with one under its controller
	forecastFound             int // a Forecaster search that put a Globe into the hand
	tripletsDies              int // a Triplets death trigger with a Triplets token on the battlefield
}

// knamedotherWatch is one game's hook state: the last decision's attackers
// and stack, and the pending Forecaster search.
type knamedotherWatch struct {
	atk       [dirac.MaxObj]bool
	stack     [dirac.MaxStack]dirac.ProjStack
	nStack    int
	searching bool
	searcher  uint8
	globes    int
}

func knamedotherName(tbl *kop.Table, c dirac.CardID) string {
	if int(c) >= len(tbl.Cards) {
		return ""
	}
	return strings.ToLower(tbl.Cards[c].Name)
}

func (w *knamedotherWatch) observe(s *dirac.State, seen *knamedotherSeen) {
	tbl := s.Table()
	var p dirac.Projection
	s.Project(&p)
	name := func(o *dirac.ProjObj) string { return knamedotherName(tbl, o.Card) }
	count := func(n string, z kop.Zone, who uint8, tokenOnly bool) int {
		m := 0
		for i := 0; i < int(p.NObj); i++ {
			o := &p.Objs[i]
			if o.Zone == z && name(o) == n && (z != kop.ZoneBattlefield || o.Ctrl == who) && (z == kop.ZoneBattlefield || o.Owner == who) && (!tokenOnly || o.Token) {
				m++
			}
		}
		return m
	}
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		atk := o.Zone == kop.ZoneBattlefield && o.Flags&dirac.ProjAttacking != 0
		if atk && !w.atk[i] && name(o) == "rufus shinra" {
			if count("darkstar", kop.ZoneBattlefield, o.Ctrl, false) == 0 {
				seen.rufusCreate++
			} else {
				seen.rufusPresent++
			}
		}
		w.atk[i] = atk
	}
	for i := int(p.NObj); i < len(w.atk); i++ {
		w.atk[i] = false
	}
	searching := false
	for i := 0; i < int(p.NStack); i++ {
		e := p.Stack[i]
		fresh := i >= w.nStack || w.stack[i] != e
		if e.Kind == 1 && knamedotherName(tbl, e.Card) == "dragonstorm forecaster" {
			searching = true
			if !w.searching {
				w.searcher = e.Ctrl
				w.globes = count("dragonstorm globe", kop.ZoneHand, e.Ctrl, false)
			}
		}
		if fresh && e.Kind == 2 && knamedotherName(tbl, e.Card) == "gruff triplets" {
			// the dies trigger: its source has left the battlefield (the
			// enters trigger's source is on it)
			src := int(e.Source)
			if src >= int(p.NObj) || p.Objs[src].Zone != kop.ZoneBattlefield {
				if count("gruff triplets", kop.ZoneBattlefield, e.Ctrl, true) > 0 {
					seen.tripletsDies++
				}
			}
		}
	}
	if w.searching && !searching && count("dragonstorm globe", kop.ZoneHand, w.searcher, false) > w.globes {
		seen.forecastFound++
	}
	w.searching = searching
	w.nStack = int(p.NStack)
	copy(w.stack[:], p.Stack[:p.NStack])
}

// knamedotherRun plays seeds 1..seeds of the matchup through tbl; it
// returns the games played, whether any diverged (the first is logged) and
// the stalls, and adds the observed events to seen.
func knamedotherRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64, seen *knamedotherSeen, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, knamedotherDecks)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"knamedother-a", "knamedother-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var w knamedotherWatch
		prev := OccupancyHook
		OccupancyHook = func(s *dirac.State) { w.observe(s, seen) }
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		OccupancyHook = prev
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div = true
			if !stopOnDiv {
				t.Logf("seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestNamedotherLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := knamedotherSetup(t)
	tbl, _, _, err := set.Matchup(knamedotherDecks[0], knamedotherDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var seen knamedotherSeen
	games, div, stalls := knamedotherRun(t, reg, set, tbl, 256, &seen, true)
	t.Logf("%d games, diverged %v, stalls %d, seen %+v", games, div, stalls, seen)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	for _, m := range []struct {
		what      string
		got, want int
	}{
		{"Rufus attacks with no Darkstar (creates one)", seen.rufusCreate, 10},
		{"Rufus attacks with Darkstar there (no trigger)", seen.rufusPresent, 10},
		{"Forecaster searches that found a named card", seen.forecastFound, 10},
		{"Triplets death triggers with a Triplets token out", seen.tripletsDies, 5},
	} {
		if m.got < m.want {
			t.Errorf("%s: %d, want at least %d", m.what, m.got, m.want)
		}
	}
}

// TestNamedotherMutation: the kernel table rewritten so that every
// PredNamed filter (all named-other here: Rufus's Darkstar gate and the
// Forecaster's search) matches nothing must make the lockstep diverge
// within 64 seeds (Rufus makes a second Darkstar; the search finds nothing).
func TestNamedotherMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := knamedotherSetup(t)
	tbl, _, _, err := set.Matchup(knamedotherDecks[0], knamedotherDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	mut := kctrnamedCopyTable(tbl)
	n := 0
	for i := range mut.Filters {
		if mut.Filters[i].Pred == kop.PredNamed {
			mut.Filters[i].PredN = -1
			n++
		}
	}
	if n == 0 {
		t.Fatal("no PredNamed filter in the matchup table")
	}
	var seen knamedotherSeen
	_, div, _ := knamedotherRun(t, reg, set, mut, 64, &seen, false)
	if !div {
		t.Fatalf("%d PredNamed filters rewritten to match nothing: no divergence in 64 seeds", n)
	}
}
