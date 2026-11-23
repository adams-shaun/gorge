package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestChosenTypeSetterLockstep plays the chosen land-type setters kgen
// lowers (Convincing Mirage, Phantasmal Terrain: RemoveLandTypes$ True with
// AddType$ ChosenType, the source's own as-enters ChooseType Basic Land
// answer; Realmwright's Continuous AddType$ ChosenType grant beside them; and
// Multiversal Passage, the entering land's own setter whose ask-then-elect
// chain queues its pay-2-life election behind the ask, the play's land count
// waiting for both) in lockstep with gorge over the choosetypesetter fixture
// pool. Both decks
// enchant lands of other types (basics, the original duals) and cast
// creatures of the old and the new colours: the enchanted land is only the
// chosen basic type, its own mana ability gone and the chosen type's
// intrinsic one in its place (CR 305.7, 305.6). The preconditions, measured
// from gorge's side, are that a setter attached and that such an enchanted
// land was tapped.
//
// The mutation drops the chosen grant (dirac.DebugChosenSetNone): an
// enchanted land keeps its old types and lacks the chosen one, so its mana
// and its type words diverge.
func TestChosenTypeSetterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runChosenSetLockstep(t, 24, true)
		t.Logf("%d games, %d divergences, %d stalls; setters attached %d, enchanted taps %d, passages entered %d", run.games, run.divs, run.stalls, st.attached, st.taps, st.passages)
		if st.attached < 4 || st.taps < 4 {
			t.Errorf("setters attached %d times, enchanted lands tapped %d times: want at least 4 each", st.attached, st.taps)
		}
		if st.passages < 2 {
			t.Errorf("Multiversal Passage entered %d times: the ask-then-elect chain never played", st.passages)
		}
	})
	t.Run("mutation-none", func(t *testing.T) {
		dirac.DebugChosenSetNone = true
		defer func() { dirac.DebugChosenSetNone = false }()
		run, _ := runChosenSetLockstep(t, 24, false)
		run.requireDivergence(t, "a chosen land-type setter never granting its choice")
	})
}

var chosenSetDecks = []fixtureDecks{{
	{{Name: "Convincing Mirage", Count: 6}, {Name: "Coral Merfolk", Count: 6}, {Name: "Typhoid Rats", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Tropical Island", Count: 4}, {Name: "Bayou", Count: 3}, {Name: "Wastes", Count: 2}},
	{{Name: "Phantasmal Terrain", Count: 6}, {Name: "Realmwright", Count: 3}, {Name: "Raging Goblin", Count: 5},
		{Name: "Savannah Lions", Count: 4}, {Name: "Multiversal Passage", Count: 3}, {Name: "Plains", Count: 6},
		{Name: "Swamp", Count: 4}, {Name: "Savannah", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 3}},
}}

// chosenSetWatch are the pool's chosen setters.
var chosenSetWatch = map[string]bool{"Convincing Mirage": true, "Phantasmal Terrain": true}

// chosenSetStats are the preconditions measured from gorge's side.
type chosenSetStats struct {
	attached int // setters attached to a land
	taps     int // lands with a chosen setter attached that were tapped
	passages int // Multiversal Passages that entered (the ask-then-elect chain)
}

// chosenSetOn reports whether the land id has a chosen setter attached.
func chosenSetOn(g *state.Game, id state.ObjID) bool {
	for _, q := range g.AliveFrom(0) {
		for _, a := range g.Zone(state.ZBattlefield, q) {
			if o := g.Obj(a); o != nil && o.AttachedTo == id && o.Face() != nil && chosenSetWatch[o.Face().Name] {
				return true
			}
		}
	}
	return false
}

// chosenSetObserve scans the events gorge appended since *seen.
func chosenSetObserve(ref *GorgeSide, seen *int, st *chosenSetStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		e := evs[*seen]
		switch e.Kind {
		case events.Attach:
			if len(e.IDs) != 1 {
				continue // attached to a player, or a re-attach without a bearer
			}
			src, land := g.Obj(e.Obj), g.Obj(e.IDs[0])
			if src == nil || src.Face() == nil || land == nil || land.Face() == nil {
				continue
			}
			if chosenSetWatch[src.Face().Name] {
				st.attached++
			}
		case events.Tap:
			if g.Obj(e.Obj) != nil && chosenSetOn(g, e.Obj) {
				st.taps++
			}
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Multiversal Passage" {
					st.passages++
				}
			}
		}
	}
}

// runChosenSetLockstep plays seeds 1..seeds of the choosetypesetter matchup,
// gorge leading, alternating the option surface by seed and measuring the
// preconditions.
func runChosenSetLockstep(t *testing.T, seeds int, report bool) (fixtureRun, chosenSetStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosetypesetter")
	if err != nil {
		t.Fatal(err)
	}
	md := chosenSetDecks[0]
	decks := [2][]*cards.Card{}
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	run := fixtureRun{casts: map[string]int{}}
	var st chosenSetStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"chosenset-a", "chosenset-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			chosenSetObserve(ref, &seen, &st)
			return nil
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, chosenSetWatch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}
