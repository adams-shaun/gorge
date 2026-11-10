package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The two cards that search outside the game, played in the Standard pool's
// own games, which carry no sideboards (the pool's kernel Config has none):
// North Wind Avatar's pick finds an empty sideboard, and Turtles Forever
// searches the library alone, an opponent choosing two of the four found.
// TestSideboardSearchLockstep (sideboard_search_test.go) plays them against
// real sideboards. The games are recorded by TestSideboardSearchRecord
// (SIDEBOARD_SEARCH_RECORD=1) on the std-tmt pool and replayed by
// TestStdSetReprosLockstep.

// sideReproCase is one recorded game: its cards and the reach it must show.
type sideReproCase struct {
	name    string
	prefer  []string
	reached func(g *GorgeSide) bool
}

var sideReproCases = []sideReproCase{
	{"turtles_forever_library_pick_chosen_by_opponent", []string{"Turtles Forever"}, turtlesForeverTookTwo},
	{"north_wind_avatar_wish_finds_empty_sideboard", []string{"North Wind Avatar"}, windAvatarWished},
}

// turtlesForeverTookTwo: Turtles Forever resolved and two legendary creature
// cards then moved from a library to a hand before anything else resolved.
func turtlesForeverTookTwo(g *GorgeSide) bool {
	in, n := false, 0
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.Resolve:
			in, n = chName(g.E.G, e.Obj) == "Turtles Forever", 0
		case in && e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZHand:
			if n++; n == 2 {
				return true
			}
		}
	}
	return false
}

// windAvatarWished: North Wind Avatar's enters trigger was put on the stack.
func windAvatarWished(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.TriggerPush && chName(g.E.G, e.Obj) == "North Wind Avatar" {
			return true
		}
	}
	return false
}

func sideReproDecks(t *testing.T, reg *cards.Registry, set cardset.Set) [2][]string {
	legends := sideLegends(t, reg, set, 0, 12)
	a := append(deckOf(4, "Turtles Forever", deckOf(4, "North Wind Avatar")...), legends...)
	a = append(a, deckOf(7, "Island", deckOf(7, "Mountain", deckOf(6, "Plains")...)...)...)
	b := append(deckOf(16, "Island"), deckOf(16, "Mountain")...)
	return [2][]string{a, append(b, legends[4:]...)}
}

// TestSideboardSearchReprosReach replays the recorded games: no divergence,
// every scripted action offered, the card's search reached.
func TestSideboardSearchReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	for _, c := range sideReproCases {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, effectLongCase{name: c.name, reached: c.reached}, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never reaches the card's search")
			}
		})
	}
}

// TestSideboardSearchRecord plays each case's policy over seeds until one
// reaches the search, and writes the repro. It runs only with
// SIDEBOARD_SEARCH_RECORD=1; the games it writes are the committed repros.
func TestSideboardSearchRecord(t *testing.T) {
	if os.Getenv("SIDEBOARD_SEARCH_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	decks := sideReproDecks(t, reg, set)
	for _, c := range sideReproCases {
		t.Run(c.name, func(t *testing.T) {
			ec := effectLongCase{name: c.name, prefer: c.prefer, decks: decks, surface: "autopay", reached: c.reached}
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: decks, Seed: seed, PassElision: true, Surface: ec.surface, Kernel: "synth std-tmt"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, ec, r, set, effectLongPolicy(m, ec, seed))
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
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
