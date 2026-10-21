package rules

// Pawprint$ (CR 700.2i, CR 107.18): a modal Charm whose root carries a
// pawprint budget -- the five BLB Seasons, "Choose up to five {P} worth of
// modes. You may choose the same mode more than once." -- must sum the
// chosen modes' own {P} costs against that budget. Pre-fix the budget was
// unread and any five modes were legal (five copies of the {P}{P}{P} mode
// spent 15 {P}). The pin is the corpus card Season of the Bold: modes cost
// 1, 2 and 3 {P} under a budget of 5.
//
// The budget rides the ONE covering KModes ask as Decision.MaxSum over each
// option's Option.Value (Decision.Validate's cumulative-budget rule), and
// the chosen modes resolve in the order they are WRITTEN on the card, not
// the order the answer named them (the Gatherer ruling; repeated copies
// keep the answer's relative order).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// seasonOfTheBoldEngine seeds the REAL corpus Season of the Bold into seat
// 0's deck (so genesis creates it and the log replays byte-for-byte),
// bridges it into hand, and returns the engine, the Config its log replays
// against, and the card's id -- the fieryConfluenceEngine shape, plus the
// Treasure token script the {P} mode's TokenScript$ resolves through.
func seasonOfTheBoldEngine(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	season, ok := reg.Lookup("Season of the Bold")
	if !ok {
		t.Fatal("corpus missing Season of the Bold")
	}
	tok, ok := reg.Token("c_a_treasure_sac")
	if !ok || tok == nil {
		t.Fatal("corpus missing c_a_treasure_sac token script")
	}
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{season}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		},
		Tokens: map[string]*cards.Card{"c_a_treasure_sac": tok},
	}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()

	var id state.ObjID
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(cand).Face().Name == "Season of the Bold" {
			id = cand
		}
	}
	if id == 0 {
		for _, cand := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(cand).Face().Name == "Season of the Bold" {
				id = cand
			}
		}
		if id == 0 {
			t.Fatal("Season of the Bold not found in seat 0's hand or library")
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
	}
	return e, cfg, id
}

// TestPawprintBudget pins the {P} budget end to end on the corpus card: the
// cast-time announcement publishes Decision.MaxSum 5 over the modes' {P}
// prices (1, 2, 3), an answer over the budget is refused by
// Decision.Validate, an answer spending exactly 5 {P} with a repeated mode
// is legal and resolves five modes, and the spell leaves the stack.
func TestPawprintBudget(t *testing.T) {
	t.Parallel()
	e, cfg, id := seasonOfTheBoldEngine(t, 1)
	addMana(t, e, 0, "RRRRR") // 3 R R

	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the cast-time KModes announcement, got %+v", d)
	}
	if !d.Repeatable {
		t.Fatal("Season of the Bold's cast-mode ask is not Repeatable -- CanRepeatModes$ True is unread")
	}
	if d.Min != 0 || d.Max != 5 {
		t.Fatalf("mode bounds = %d..%d, want 0..5 (MinCharmNum$ 0, CharmNum$ 5)", d.Min, d.Max)
	}
	if d.MaxSum != 5 {
		t.Fatalf("cast-mode ask budget = %d, want 5 (root Pawprint$ 5) -- the {P} budget is unread", d.MaxSum)
	}
	if len(d.Options) != 3 {
		t.Fatalf("mode options = %d, want 3: %+v", len(d.Options), d.Options)
	}
	for i, want := range []int{1, 2, 3} {
		if got := d.Options[i].Value; got != want {
			t.Fatalf("mode %d Option.Value = %d, want %d (its own Pawprint$) -- %+v", i, got, want, d.Options[i])
		}
	}
	tok, dmg := 0, 2

	// Five copies of the {P}{P}{P} mode spend 15 {P} against a budget of 5:
	// Decision.Validate must refuse the intent (the wire contract the seat
	// re-answers through), and the ask must survive to accept a legal one.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{dmg, dmg, dmg, dmg, dmg}}); err == nil {
		t.Fatal("five copies of the 3-{P} mode passed Validate -- the {P} budget is not enforced")
	} else if !strings.Contains(err.Error(), "exceeds the budget") {
		t.Fatalf("over-budget submit error = %v, want the cumulative-budget rejection", err)
	}
	// Five copies of the 1-{P} mode spend exactly the budget: legal, with
	// the repeat, and all five resolve.
	submitChoices(t, e, tok, tok, tok, tok, tok)
	passUntilStackEmpty(t, e, 20)
	if n := len(battlefieldTokens(e, 0)); n != 5 {
		t.Fatalf("seat 0 tokens = %d, want 5 (five 1-{P} picks at exactly the budget)", n)
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Errorf("Season of the Bold resolved to %s, want Graveyard", z)
	}
	replayCheck(t, e, cfg)
}

// TestModeDecisionPublishesPresentZeroBudget pins the budget's PRESENCE, not
// just its value (CR 700.2i): a root Pawprint$ 0 is a real budget -- the ask
// publishes Decision.Budgeted with MaxSum 0, HasBudget holds, and the shared
// cumulative-budget rule refuses any priced pick -- while an absent root
// Pawprint$ publishes no budget at all. Pre-fix a present zero read as
// absent (MaxSum 0 alone means "no budget").
func TestModeDecisionPublishesPresentZeroBudget(t *testing.T) {
	t.Parallel()
	svars := map[string]string{
		"mA": "DB$ Draw | NumCards$ 1 | Pawprint$ 1",
		"mB": "DB$ Draw | NumCards$ 2 | Pawprint$ 0",
	}
	zero := &cards.SA{API: "Charm", Params: map[string]string{
		"Choices": "mA,mB", "Pawprint": "0",
		"CharmNum": "5", "MinCharmNum": "0", "CanRepeatModes": "True"}}
	d := modeDecisionForChoices(nil, 0, 1, zero, svars, []string{"mA", "mB"}, 0, 5, true)
	d.Seq = 7
	if d.MaxSum != 0 || !d.Budgeted || !d.HasBudget() {
		t.Fatalf("Pawprint$ 0 ask: MaxSum %d Budgeted %v HasBudget %v -- a present zero budget must stay live", d.MaxSum, d.Budgeted, d.HasBudget())
	}
	if d.Options[0].Value != 1 || d.Options[1].Value != 0 {
		t.Fatalf("Pawprint$ 0 ask option values = %v, want [1 0]", d.Options)
	}
	if err := d.Validate(decision.Intent{Seq: 7, Player: 0, Choices: []int{0}}); err == nil {
		t.Fatal("a 1-{P} pick was accepted against a present zero budget")
	}
	if err := d.Validate(decision.Intent{Seq: 7, Player: 0, Choices: []int{1, 1}}); err != nil {
		t.Fatalf("two free picks were refused against a present zero budget: %v", err)
	}
	absent := &cards.SA{API: "Charm", Params: map[string]string{
		"Choices": "mA,mB", "CharmNum": "5", "MinCharmNum": "0", "CanRepeatModes": "True"}}
	d2 := modeDecisionForChoices(nil, 0, 1, absent, svars, []string{"mA", "mB"}, 0, 5, true)
	if d2.HasBudget() || d2.Budgeted || d2.MaxSum != 0 {
		t.Fatalf("absent Pawprint$ ask: MaxSum %d Budgeted %v HasBudget %v -- want no budget", d2.MaxSum, d2.Budgeted, d2.HasBudget())
	}
}

// TestPawprintModesResolveInPrintedOrder pins CR 700.2i's resolution order:
// the instructions are followed in the order the modes are WRITTEN on the
// card, not the order the answer named them. The answer names the 2-{P}
// exile mode first and the 1-{P} Treasure mode second; the Treasure must be
// created BEFORE the top two cards are exiled.
func TestPawprintModesResolveInPrintedOrder(t *testing.T) {
	t.Parallel()
	e, cfg, id := seasonOfTheBoldEngine(t, 1)
	addMana(t, e, 0, "RRRRR")

	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the cast-time KModes announcement, got %+v", d)
	}
	tok := 0
	exile := 1
	for i, o := range d.Options {
		if o.Value == 1 {
			tok = i
		}
		if o.Value == 2 {
			exile = i
		}
	}
	if d.Options[tok].Value != 1 || d.Options[exile].Value != 2 {
		t.Fatalf("mode prices = %+v, want 1 then 2 by printed order", d.Options)
	}
	submitChoices(t, e, exile, tok)
	passUntilStackEmpty(t, e, 20)

	if n := len(battlefieldTokens(e, 0)); n != 1 {
		t.Fatalf("seat 0 tokens = %d, want 1", n)
	}
	exiled := 0
	for _, cand := range e.G.Zone(state.ZExile, 0) {
		if o := e.G.Obj(cand); o != nil && !o.IsToken {
			exiled++
		}
	}
	if exiled != 2 {
		t.Fatalf("seat 0 exiled cards = %d, want 2", exiled)
	}
	// The chosen order is exiled-then-token; the printed order (Choices$
	// DBToken, DBExile) resolves the token FIRST.
	modeAt := -1
	for i, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen && ev.Text != "" {
			modeAt = i
		}
	}
	if modeAt < 0 {
		t.Fatal("no ModeChosen marker recorded")
	}
	tokAt, exileAt := -1, -1
	for i, ev := range e.L.Events {
		if i < modeAt {
			continue
		}
		switch {
		case ev.Kind == events.TokenCreate && tokAt < 0:
			tokAt = i
		case ev.Kind == events.MoveZone && ev.From == state.ZLibrary && ev.To == state.ZExile && exileAt < 0:
			exileAt = i
		}
	}
	if tokAt < 0 || exileAt < 0 {
		t.Fatalf("token/exile events after the answer not found: token %d exile %d", tokAt, exileAt)
	}
	if tokAt > exileAt {
		t.Fatalf("resolution followed the ANSWER order (exile at %d before token at %d) -- CR 700.2i's printed order is not applied", exileAt, tokAt)
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Errorf("Season of the Bold resolved to %s, want Graveyard", z)
	}
	replayCheck(t, e, cfg)
}
