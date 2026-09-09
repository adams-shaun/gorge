package host

import (
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/view"
)

// This file builds the host match the bounds coverage needs but
// fourSeatTable (seed 99) cannot provide: a finished game whose stream
// contains a MID-RESOLUTION ask. sampleLoader's four decks are all two-drops
// with no modal spell, no as-enters choice and no unless-pay, so a match
// over them has no ask->priority adjacency and boundsOf's derivation is
// never exercised against the suspended path. The deck below keeps the
// sample shape (lands, a vanilla creature so combat ends the game, a modal
// charm) so the bot plays a real game that asks a modal "modes" question
// mid-resolution, which is exactly the shape the old pass-branch emit lied
// about (see rules/legal.go and rules/resolution.go).

const modalCharmSrc = "Name:My Charm\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ Charm | Choices$ DoA,DoB\n" +
	"SVar:DoA:DB$ GainLife | Defined$ You | LifeAmount$ 3 | SpellDescription$ gain 3\n" +
	"SVar:DoB:DB$ LoseLife | Defined$ You | LifeAmount$ 3 | SpellDescription$ lose 3\nOracle:x\n"

const modalWhelpSrc = "Name:My Whelp\nManaCost:R\nTypes:Creature Whelp\nPT:2/2\nOracle:x\n"

// modalCard parses one card script inline (cards.ParseBytes + Link +
// ApplyIntrinsics, the same shape testutil.parseCard uses), so the fixture
// stays corpus-free: no GPL .cards, no file.
func modalCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("modal_fixture_test.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse %q: %v", src, diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// modalBattleDeck is a 40-card deck of mountains plus a vanilla 2/2 whelp
// plus copies of the modal charm: enough lands to cast everything, enough
// creatures for combat to actually end the game (a pure lands-and-charm deck
// just decks out after 100+ turns), and a modal spell the bot casts, whose
// mid-resolution KModes ask is the point of the fixture.
func modalBattleDeck(t *testing.T) []*cards.Card {
	t.Helper()
	land := modalCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	charm := modalCard(t, modalCharmSrc)
	whelp := modalCard(t, modalWhelpSrc)
	out := make([]*cards.Card, 0, 40)
	for i := 0; i < 20; i++ {
		out = append(out, land)
	}
	for i := 0; i < 8; i++ {
		out = append(out, charm)
	}
	for i := 0; i < 12; i++ {
		out = append(out, whelp)
	}
	return out
}

// modalLoader serves the four modal decks under the fourSeatTable names.
func modalLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	deck := modalBattleDeck(t)
	byName := map[string][]*cards.Card{
		"a": deck, "b": deck, "c": deck, "d": deck,
	}
	return func(name string) (Deck, error) {
		cs, ok := byName[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: cs}, nil
	}
}

// modalTable is the table the mid-resolution fixture plays. Seed 2 (the
// match's own MatchSeed) drives a game that reaches a mid-resolution
// "modes" ask within a game that combat still ends in a modest number of
// turns, so the leaf proves boundsOf against the suspended path without
// costing a minutes-long match.
func modalTable(id TableID) TableConfig {
	return TableConfig{ID: id, Name: "Table " + string(id), Seats: 4,
		Decks: []string{"a", "b", "c", "d"}, Seed: 2, Pace: 0,
		Spectator: view.Omniscient}
}

// modalOptions gives a test its own single match slot plus the modal deck
// loader (testOptions hardcodes the sample loader, which has no modal spell).
func modalOptions(t *testing.T) Options {
	t.Helper()
	takeMatchSlot(t)
	return Options{LoadDeck: modalLoader(t), Sleep: func(time.Duration, <-chan struct{}) {}}
}

// modalFinishedTable plays the modal match ONCE per test binary (a pure
// function of the modal table config, so the cost is paid once) and hands
// back a reader-only *match, mirroring finishedTable's shared-fixture shape.
// Callers read m.e.L and m.bounds; they must not mutate the match or the
// registry. The registry is closed by TestMain, not by the caller.
var modalFixture struct {
	once sync.Once
	mu   sync.Mutex
	r    *Registry
	m    *match
	err  error
}

func modalFinishedTable(t *testing.T) *match {
	t.Helper()
	modalFixture.once.Do(func() {
		r, err := New(modalOptions(t))
		if err != nil {
			modalFixture.err = err
			return
		}
		if err := r.AddTable(modalTable("t2")); err != nil {
			r.Close()
			modalFixture.err = err
			return
		}
		if err := r.Start("t2"); err != nil {
			r.Close()
			modalFixture.err = err
			return
		}
		r.Wait("t2")
		r.mu.RLock()
		tb := r.tables["t2"]
		r.mu.RUnlock()
		modalFixture.mu.Lock()
		modalFixture.r, modalFixture.m = r, tb.history[0]
		modalFixture.mu.Unlock()
	})
	if modalFixture.err != nil {
		t.Fatalf("shared mid-resolution finished table: %v", modalFixture.err)
	}
	return modalFixture.m
}

// matchHasDecisionAsk reports whether the log carries a DecisionAsk with the
// given text -- used to prove a fixture actually exercises the shape a leaf
// is about, so a vacuous green (a fixture that drifted to no such ask) reads
// as a failure rather than a pass.
func matchHasDecisionAsk(evs []events.Event, text string) bool {
	for _, ev := range evs {
		if ev.Kind == events.DecisionAsk && ev.Text == text {
			return true
		}
	}
	return false
}
