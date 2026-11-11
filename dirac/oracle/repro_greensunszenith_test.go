package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Green Sun's Zenith: search your library for a green creature card with mana
// value X or less, put it onto the battlefield, then shuffle; then shuffle
// the spell itself into its owner's library (CR 107.3, 608.2n, 701.23). kgen
// lowers it as a Search over a mana-value <= X filter followed by
// ExtSpellToLib (the Zenith shape Blue Sun's Zenith already rides).
//
// The deck holds green creatures of mana value 1, 2 and 6 and a red one: the
// search menu is bounded by the X paid and by colour, so a filter that lost
// either diverges at the fetch. The game is recorded by
// TestReproGreenSunsZenithRecord (GREEN_SUNS_ZENITH_RECORD=1) and replayed by
// TestReproGreenSunsZenithReach, which requires the fetch and the spell's
// return to its owner's library to happen.
func greenSunsZenithCase() effectLongCase {
	mine := deckOf(8, "Green Sun's Zenith")
	mine = append(mine, deckOf(3, "Llanowar Elves")...)
	mine = append(mine, deckOf(3, "Grizzly Bears")...)
	mine = append(mine, deckOf(2, "Craw Wurm")...)
	mine = append(mine, deckOf(2, "Hill Giant")...)
	mine = append(mine, deckOf(14, "Forest")...)
	return effectLongCase{
		name:   "repro_green_suns_zenith_fetches_then_shuffles_itself",
		prefer: []string{"Green Sun's Zenith"},
		decks: [2][]string{
			mine,
			append(deckOf(16, "Forest"), deckOf(8, "Grizzly Bears")...),
		},
		surface: "full",
		reached: greenSunsZenithResolved,
	}
}

// greenSunsZenithResolved reads the finished gorge log: a green creature card
// of the first player's moves from their library to the battlefield, and then
// a Green Sun's Zenith moves from the stack to its owner's library.
func greenSunsZenithResolved(g *GorgeSide) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	fetched := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.MoveZone {
			continue
		}
		switch {
		case e.From == state.ZLibrary && e.To == state.ZBattlefield:
			switch name(e.Obj) {
			case "Llanowar Elves", "Grizzly Bears", "Craw Wurm":
				fetched = true
			}
		case e.From == state.ZStack && e.To == state.ZLibrary && name(e.Obj) == "Green Sun's Zenith" && fetched:
			return true
		}
	}
	return false
}

// TestReproGreenSunsZenithReach replays the recorded game: no divergence,
// every scripted action offered, and the Zenith fetches a creature and
// shuffles itself away.
func TestReproGreenSunsZenithReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := greenSunsZenithCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never fetches a creature with Green Sun's Zenith and shuffles the spell into the library")
	}
}

// TestReproGreenSunsZenithRecord plays seeds until one reaches the
// resolution, and writes the repro. It runs only with
// GREEN_SUNS_ZENITH_RECORD=1; the game it writes is the committed repro.
func TestReproGreenSunsZenithRecord(t *testing.T) {
	if os.Getenv("GREEN_SUNS_ZENITH_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := greenSunsZenithCase()
	for seed := uint64(1); seed <= 200; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
			Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed reached Green Sun's Zenith's resolution")
}
