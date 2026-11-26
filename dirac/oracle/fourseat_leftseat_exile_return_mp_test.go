package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// A seat that loses to combat damage leaves the game in the state-based pass
// AFTER the damage pass (CR 704.3b, 800.4a), and an "exile until this leaves
// the battlefield" host it controlled returns its exiled card there (CR
// 610.3d). The returned card's as-enters choice is a real ask: the combat
// pass must be covered by a tape run (the pass gate counts the pending
// return), the run must still be inside its resolution when the sweep
// returns the card (the run ends at completeCombatPass, after the SBA tail,
// not at the damage loop's end), and the return follows the WHOLE departure
// (ceaseDepartedObjects batches its zone moves), so the ask is posed after
// every owned object has left.
//
// left_seat_exile_return_asks_four_seat_pod is pair 830 (kfuzz -kernel cmd5
// -seats 4, autopay surface, seed 6168047488322616863): a Grasp of Fate holds
// an Outpost Siege in exile while its controller loses to combat damage. The
// kernel poses the Siege's Khans-or-Dragons choice to its owner; gorge used
// to answer it with its default ("no resolution run serves it (modes/modes)")
// and offer a priority pass where the kernel offered the two modes, diverging
// at decision 2368 (implicating Scourge of Valkas, the board's other
// until-leaves exile holder).
func TestFourSeatLeftSeatExileReturnAsksLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const name = "left_seat_exile_return_asks_four_seat_pod"
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoCorpus) || errors.Is(err, cardset.ErrNoTables) {
		t.Skipf("no cmd5 table or corpus (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "one-off" {
		t.Fatalf("pool %q, want one-off", r.Pool)
	}
	if len(r.Cards) != 2 || len(r.MoreCards) != 2 {
		t.Fatalf("recording has %d+%d decks, want a four-seat pod", len(r.Cards), len(r.MoreCards))
	}
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	if m.NSeat() != 4 {
		t.Fatalf("matchup has %d seats, want 4", m.NSeat())
	}
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(reg, set, names))
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	g, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, r.Seed, g, k, &ScriptPolicy{Actions: r.Actions}, rc)
	// The recorded game's ask must now be served, never defaulted: no Note
	// may name a run that failed to serve it.
	for _, ev := range g.E.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "no resolution run serves it") {
			t.Errorf("gorge logged %q: an as-enters ask was answered with its default", ev.Text)
		}
	}
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= 2368 {
		t.Errorf("%d decisions, the game must go past decision 2368", res.Decisions)
	}
}
