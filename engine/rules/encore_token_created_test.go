package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// freshEncoreRegistryWith compiles a minimal corpus fresh (no ir.gob.gz cache)
// -- the real Impulsive Pilferer script, the Treasure token its dies trigger
// mints, and the caller's extra corpus cards and token scripts -- copied out
// of the worktree's .cards corpus into a temp dir. A fresh compile is the only
// way a test can see the K:Encore expansion before the shared cache is
// recompiled (see altcast_test.go's file header).
func freshEncoreRegistryWith(t *testing.T, extraCards, extraTokens []string) *cards.Registry {
	t.Helper()
	src := testutil.CorpusDirOrSkip(t)
	dir := t.TempDir()
	cf := filepath.Join(dir, "cardsfolder")
	tk := filepath.Join(dir, "tokenscripts")
	if err := os.MkdirAll(cf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tk, 0o755); err != nil {
		t.Fatal(err)
	}
	type pair struct{ from, to string }
	pairs := []pair{
		{filepath.Join(cards.CorpusDir(src), "i", "impulsive_pilferer.txt"), filepath.Join(cf, "impulsive_pilferer.txt")},
		{filepath.Join(src, "tokenscripts", "c_a_treasure_sac.txt"), filepath.Join(tk, "c_a_treasure_sac.txt")},
	}
	for _, rel := range extraCards {
		pairs = append(pairs, pair{filepath.Join(cards.CorpusDir(src), rel), filepath.Join(cf, filepath.Base(rel))})
	}
	for _, rel := range extraTokens {
		pairs = append(pairs, pair{filepath.Join(src, "tokenscripts", rel), filepath.Join(tk, filepath.Base(rel))})
	}
	for _, p := range pairs {
		b, err := os.ReadFile(p.from)
		if err != nil {
			t.Fatalf("corpus copy: %v", err)
		}
		if err := os.WriteFile(p.to, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg, _, err := cards.CompileDir(cf)
	if err != nil {
		t.Fatalf("fresh compile: %v", err)
	}
	return reg
}

// activateEncore submits the pending priority decision's Encore ability option
// for the graveyard card id.
func activateEncore(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no encore ability option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
}

// TestEncoreTokenFiresTokenCreatedMirkwoodBats pins CR 702.141a/111.1: Encore
// creates token copies, so a "whenever you create a token" watcher fires. The
// Encore mint is events.CardToken, which lands the token on the battlefield
// with no MoveZone, so before the fix the TokenCreated mask and matcher never
// saw it and the opponent kept their life total.
func TestEncoreTokenFiresTokenCreatedMirkwoodBats(t *testing.T) {
	t.Parallel()
	reg := freshEncoreRegistryWith(t, []string{"m/mirkwood_bats.txt"}, nil)
	e, cfg, _ := altCostEngineReg(t, 917, reg, []string{"Impulsive Pilferer", "Mirkwood Bats"}, nil, nil)

	batsID := findCardObj(t, e, 0, "Mirkwood Bats", state.ZBattlefield)
	pilID := findCardObj(t, e, 0, "Impulsive Pilferer", state.ZGraveyard)
	if o := e.G.Obj(batsID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Mirkwood Bats not on seat 0's battlefield: %+v", o)
	}
	if o := e.G.Obj(pilID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Impulsive Pilferer not in seat 0's graveyard: %+v", o)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Impulsive Pilferer"); got != 0 {
		t.Fatalf("precondition: seat 0 already has %d Pilferer tokens", got)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", got)
	}

	addMana(t, e, 0, "CCCR")
	activateEncore(t, e, pilID)
	passUntilStackEmpty(t, e, 40)

	if got := countTokensNamedOnSeat(t, e, 0, "Impulsive Pilferer"); got != 1 {
		t.Fatalf("encore tokens = %d, want 1", got)
	}
	// Check life before the end step: the sacrifice fires Bats' Secondary$
	// trigger and would drop it to 18.
	if got := e.G.Players[1].Life; got != 19 {
		t.Fatalf("seat 1 life = %d, want 19 (TokenCreated did not fire for the CardToken mint)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEncoreTokenOnceLatchWithAkimMakesOneBird pins the TokenCreatedOnce
// batch latch against Encore: three opponent token mints in one resolution
// queue Akim's line once, so exactly one Bird appears.
func TestEncoreTokenOnceLatchWithAkimMakesOneBird(t *testing.T) {
	t.Parallel()
	reg := freshEncoreRegistryWith(t, []string{"a/akim_the_soaring_wind.txt"}, []string{"w_1_1_bird_flying.txt"})
	pilferer, ok := reg.Lookup("Impulsive Pilferer")
	if !ok {
		t.Fatal("fresh registry lacks Impulsive Pilferer")
	}
	akim, ok := reg.Lookup("Akim, the Soaring Wind")
	if !ok {
		t.Fatal("fresh registry lacks Akim, the Soaring Wind")
	}
	decks := [][]*cards.Card{
		append([]*cards.Card{pilferer, akim}, mountainDeck(t, 38)...),
		mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40),
	}
	cfg := seatZeroStart(Config{Seed: 918, Names: []string{"a", "b", "c", "d"},
		Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()

	akimID := findCardObj(t, e, 0, "Akim, the Soaring Wind", state.ZBattlefield)
	pilID := findCardObj(t, e, 0, "Impulsive Pilferer", state.ZGraveyard)
	if o := e.G.Obj(akimID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Akim not on seat 0's battlefield: %+v", o)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 0 {
		t.Fatalf("precondition: seat 0 already has %d Bird Tokens", got)
	}

	addMana(t, e, 0, "CCCR")
	activateEncore(t, e, pilID)
	passUntilStackEmpty(t, e, 80)

	if got := countTokensNamedOnSeat(t, e, 0, "Impulsive Pilferer"); got != 3 {
		t.Fatalf("encore tokens = %d, want 3 (one per opponent)", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 1 {
		t.Fatalf("Bird Tokens = %d, want 1 (TokenCreatedOnce must latch the batch)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEncoreTokenOpponentBatsDoesNotFire is the negative control: a Mirkwood
// Bats controlled by the OPPONENT does not fire for seat 0's Encore token,
// because ValidPlayer$ You reads ev.Player (the creator), not the watcher's
// controller.
func TestEncoreTokenOpponentBatsDoesNotFire(t *testing.T) {
	t.Parallel()
	reg := freshEncoreRegistryWith(t, []string{"m/mirkwood_bats.txt"}, nil)
	pilferer, ok := reg.Lookup("Impulsive Pilferer")
	if !ok {
		t.Fatal("fresh registry lacks Impulsive Pilferer")
	}
	bats, ok := reg.Lookup("Mirkwood Bats")
	if !ok {
		t.Fatal("fresh registry lacks Mirkwood Bats")
	}
	decks := [][]*cards.Card{
		append([]*cards.Card{pilferer}, mountainDeck(t, 39)...),
		append([]*cards.Card{bats}, mountainDeck(t, 39)...),
	}
	cfg := seatZeroStart(Config{Seed: 919, Names: []string{"a", "b"}, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()

	batsID := findCardObj(t, e, 1, "Mirkwood Bats", state.ZBattlefield)
	pilID := findCardObj(t, e, 0, "Impulsive Pilferer", state.ZGraveyard)
	if o := e.G.Obj(batsID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: Mirkwood Bats not on seat 1's battlefield: %+v", o)
	}
	if o := e.G.Obj(pilID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Impulsive Pilferer not in seat 0's graveyard: %+v", o)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", got)
	}

	addMana(t, e, 0, "CCCR")
	activateEncore(t, e, pilID)
	passUntilStackEmpty(t, e, 40)

	if got := countTokensNamedOnSeat(t, e, 0, "Impulsive Pilferer"); got != 1 {
		t.Fatalf("encore tokens = %d, want 1", got)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 life = %d, want 20 (the opponent's Bats fired for seat 0's token)", got)
	}
	replayCheck(t, e, cfg)
}
