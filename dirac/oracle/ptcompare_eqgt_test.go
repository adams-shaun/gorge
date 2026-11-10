package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Amalia Benavides Aguirre (destroy the other creatures if her power is
// exactly 20) and Hulkling, Burgeoning Bruiser (a +1/+1 counter when another
// creature you control enters with greater power or toughness) are the
// lowered power == N and toughness > a value filters. TestStdSetReprosLockstep
// replays one recorded game of each; ptEqGtRepros requires the recorded game
// to reach the card's effect, since a lockstep entry alone would pass a
// script that never cast it.
//
// The games are recorded by TestPTCompareEqGtRecord (PT_COMPARE_RECORD=1) on
// the scripted policy of charmRepeatPolicy: both seats cast the case's cards
// first, after the card itself.
type ptEqGtCase struct {
	charmRepeatCase
	// count is the events of its kind whose object is of the named card (any
	// object when named empty) in the recorded game; the game needs n of them.
	of string
}

func ptEqGtCases() []ptEqGtCase {
	return []ptEqGtCase{
		{charmRepeatCase: charmRepeatCase{
			// Hulkling (2/3) beside a 3/3 (greater power), a 2/4 (greater
			// toughness) and a 0/3 (neither): the counters land on Hulkling
			name: "pt_compare_hulkling_greater_power_or_toughness", card: "Hulkling, Burgeoning Bruiser",
			prefer: []string{"Ultron, Artificial Malevolence", "Hercules, Prince of Power", "Rick Jones, Destined Sidekick"},
			decks: [2][]string{
				append(deckOf(6, "Hulkling, Burgeoning Bruiser"), append(deckOf(4, "Ultron, Artificial Malevolence"), append(deckOf(4, "Hercules, Prince of Power"), append(deckOf(3, "Rick Jones, Destined Sidekick"), deckOf(11, "Forest")...)...)...)...),
				deckOf(24, "Forest"),
			},
			kind: events.CounterChange, n: 2,
		}, of: "Hulkling, Burgeoning Bruiser"},
		{charmRepeatCase: charmRepeatCase{
			// Amalia explores whenever her controller gains life, and her power
			// is not 20, so nothing is destroyed
			name: "pt_compare_amalia_explores_power_not_twenty", card: "Amalia Benavides Aguirre", prefer: []string{"Revitalize"},
			decks: [2][]string{
				append(deckOf(6, "Amalia Benavides Aguirre"), append(deckOf(6, "Revitalize"), append(deckOf(6, "Plains"), deckOf(6, "Swamp")...)...)...),
				deckOf(24, "Forest"),
			},
			kind: events.Explore, n: 2,
		}, of: "Amalia Benavides Aguirre"},
	}
}

func ptEqGtPlay(t *testing.T, reg *cards.Registry, c ptEqGtCase, r *Repro, set cardset.Set, pol Policy) (Result, int) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000 // recording
	}
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	hits := 0
	rc.Finish = func(g *GorgeSide) {
		for _, e := range g.E.L.Events {
			if e.Kind != c.kind {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == c.of {
				hits++
			}
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), hits
}

// TestPTCompareEqGtReprosResolve replays the recorded games: no divergence,
// every scripted action offered, and the card's effect happened n times.
func TestPTCompareEqGtReprosResolve(t *testing.T) {
	reg := synthReg(t)
	for _, c := range ptEqGtCases() {
		t.Run(c.card, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hits := ptEqGtPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if hits < c.n {
				t.Fatalf("%d %v events of %s, want at least %d", hits, c.kind, c.of, c.n)
			}
		})
	}
}

// TestPTCompareEqGtRecord plays the cases' scripted policies over seeds until
// one reaches the card's effect n times, and writes the repro. It runs only
// with PT_COMPARE_RECORD=1; the games it writes are the committed repros.
func TestPTCompareEqGtRecord(t *testing.T) {
	if os.Getenv("PT_COMPARE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range ptEqGtCases() {
		t.Run(c.card, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, hits := ptEqGtPlay(t, reg, c, r, set, charmRepeatPolicy(m, c.charmRepeatCase))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if hits < c.n {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, %d events", seed, len(res.Actions), hits)
				return
			}
			t.Fatalf("no seed reached %s's effect %d times", c.card, c.n)
		})
	}
}
