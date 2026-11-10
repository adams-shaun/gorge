package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Legions to Ashes exiles its target and every token with the target's name
// that the target's controller controls. The lowered sweep (kgen
// TestChangeZoneAllSameNameLowers, dirac TestSameNameT0TokensBattlefield) is
// played here in a game both engines play decision by decision:
// TestStdSetReprosLockstep replays the repro, and
// TestLegionsToAshesReproExilesTheTokens requires the recorded game to cast
// the card and exile at least two battlefield permanents in its resolution,
// since a lockstep entry alone would pass a script that never cast it.
//
// The game is recorded by TestLegionsToAshesRecord (LEGIONS_RECORD=1, the
// kernel table built from the decks): seat 0 casts Legions to Ashes, seat 1
// casts Raise the Alarm (two Soldier tokens), targets are taken in menu order
// and everything else passes.
const legionsRepro = "sweep_exiles_the_target_and_same_named_tokens_legions_to_ashes"

var legionsDecks = [2][]string{
	append(deckOf(8, "Legions to Ashes"), append(deckOf(8, "Plains"), deckOf(8, "Swamp")...)...),
	append(deckOf(8, "Raise the Alarm"), deckOf(16, "Plains")...),
}

func legionsPolicy(m *Matchup) Policy {
	return charmRepeatPolicy(m, charmRepeatCase{card: "Legions to Ashes", prefer: []string{"Raise the Alarm"}})
}

// legionsExiled counts the battlefield permanents exiled between Legions to
// Ashes' resolution and the next resolution.
func legionsExiled(evs []events.Event, g *state.Game) (hits int) {
	res := false
	for i := range evs {
		e := &evs[i]
		switch {
		case e.Kind == events.Resolve && res:
			return hits
		case e.Kind == events.Resolve:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Legions to Ashes" {
				res = true
			}
		case res && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZExile:
			hits++
		}
	}
	return hits
}

func legionsPlay(t *testing.T, reg *cards.Registry, r *Repro, set cardset.Set, pol Policy) (Result, int) {
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
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	var hits int
	rc.Finish = func(g *GorgeSide) { hits = legionsExiled(g.E.L.Events, g.E.G) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), hits
}

// TestLegionsToAshesReproExilesTheTokens replays the recorded game: no
// divergence, every scripted action offered, and the sweep exiles its target
// and at least one same-named token.
func TestLegionsToAshesReproExilesTheTokens(t *testing.T) {
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", legionsRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, hits := legionsPlay(t, reg, r, charmRepeatSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if hits < 2 {
		t.Fatalf("Legions to Ashes exiled %d permanents, want the target and at least one same-named token", hits)
	}
}

// TestLegionsToAshesRecord plays the scripted policy over seeds until Legions
// to Ashes exiles two or more permanents and writes the repro. It runs only
// with LEGIONS_RECORD=1.
func TestLegionsToAshesRecord(t *testing.T) {
	if os.Getenv("LEGIONS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	set := charmRepeatSet(t, legionsDecks)
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{legionsRepro + "_0", legionsRepro + "_1"}, Cards: legionsDecks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, hits := legionsPlay(t, reg, r, set, legionsPolicy(m))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if hits < 2 {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", legionsRepro+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions, %d exiled", seed, len(res.Actions), hits)
		return
	}
	t.Fatal("no seed exiled two permanents with Legions to Ashes")
}
