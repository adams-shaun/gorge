package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// tokenFixtures builds this file's own token cards -- authored here for the
// test, never copied from the corpus's own (GPL-3.0) tokenscripts under
// .cards/tokenscripts. Names and stems echo real Forge shapes (a Goblin
// token and a Deathtouch Wurm) closely enough to read naturally in a test
// failure, but the text is original.
func tokenFixtures(t *testing.T) map[string]*cards.Card {
	t.Helper()
	return map[string]*cards.Card{
		"r_1_1_goblin":                      mkCard(t, "Name:Goblin Token\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"),
		"c_3_3_a_phyrexian_wurm_deathtouch": mkCard(t, "Name:Phyrexian Wurm Token\nTypes:Creature Phyrexian Wurm\nPT:3/3\nK:Deathtouch\nOracle:x\n"),
	}
}

// fixtureHostWithTokens is fixtureHost (context_test.go), a 2-seat game with
// one object already on it, plus Game.Tokens populated with tokenFixtures --
// what every effToken test in this file needs to have something to mint.
func fixtureHostWithTokens(t *testing.T) (*fakeHost, *Ctx) {
	t.Helper()
	h, c := fixtureHost(t)
	h.g.Tokens = tokenFixtures(t)
	return h, c
}

// countKind reports how many events of kind k are in h's captured log --
// this file's own two-argument variant of rules/replacement_updated_test.go's
// countKind(log, kind, id): effToken's tests care how many tokens got
// minted in total, not which one object a particular event names.
func countKind(h *fakeHost, k events.Kind) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == k {
			n++
		}
	}
	return n
}

func TestTokenCreatesEachScriptTheGivenNumberOfTimes(t *testing.T) {
	h, c := fixtureHostWithTokens(t) // Game.Tokens: r_1_1_goblin, c_3_3_a_phyrexian_wurm_deathtouch
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenAmount": "2", "TokenScript": "r_1_1_goblin", "TokenOwner": "You"}})
	if n := countKind(h, events.TokenCreate); n != 2 {
		t.Fatalf("%d TokenCreate events", n)
	}
	bf := h.Game().Zone(state.ZBattlefield, c.Controller)
	if len(bf) != 2 || !h.Game().Obj(bf[0]).IsToken || h.Game().Obj(bf[0]).Face().Name != "Goblin Token" {
		t.Fatalf("battlefield %v", bf)
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "c_3_3_a_phyrexian_wurm_deathtouch,r_1_1_goblin", "RememberTokens": "True"}})
	if len(c.Remembered) != 2 || len(h.Game().Zone(state.ZBattlefield, c.Controller)) != 4 {
		t.Fatalf("remembered %v", c.Remembered)
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "no_such"}})
	if countKind(h, events.TokenCreate) != 4 {
		t.Fatal("unknown script created something")
	}
}

// TestTokenUnknownScriptNotesAndCreatesNothing pins down the exact totality
// behaviour TestTokenCreatesEachScriptTheGivenNumberOfTimes only checks the
// count for: an unrecognised TokenScript$ stem is a Note diagnostic naming
// the stem, not a silent no-op and never a panic (Resolve's own convention
// for every other unimplemented/unrecognised input in this package).
func TestTokenUnknownScriptNotesAndCreatesNothing(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "no_such"}})
	if n := countKind(h, events.TokenCreate); n != 0 {
		t.Fatalf("%d TokenCreate events, want 0", n)
	}
	if len(h.log) != 1 || h.log[0].Kind != events.Note || h.log[0].Text != "unknown token script no_such" {
		t.Fatalf("log = %+v", h.log)
	}
	if bf := h.Game().Zone(state.ZBattlefield, c.Controller); len(bf) != 0 {
		t.Fatalf("battlefield = %v, want empty", bf)
	}
}

// TestTokenAmountZeroCreatesNothing: TokenAmount$ 0 is a legal (if useless)
// value. This checks the explicit-zero path specifically, which is distinct
// from Num's own default: a MISSING TokenAmount$ falls back to 1 (Num's def
// parameter, which TestTokenCreatesEachScriptTheGivenNumberOfTimes already
// exercises with TokenAmount$ absent on its RememberTokens$ call) -- an
// unparseable-but-present value falls to Num's own zero fallback instead
// (Num returns 0 for a present-but-unparseable value, not the default; only
// a MISSING key returns the default), which happens to read the same as
// this test's explicit "0" but is a different code path and not what this
// test is pinning down.
func TestTokenAmountZeroCreatesNothing(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenAmount": "0", "TokenScript": "r_1_1_goblin"}})
	if n := countKind(h, events.TokenCreate); n != 0 {
		t.Fatalf("%d TokenCreate events, want 0", n)
	}
	if len(h.log) != 0 {
		t.Fatalf("log = %+v, want empty (TokenAmount$ 0 is not an unknown script -- no Note either)", h.log)
	}
	if bf := h.Game().Zone(state.ZBattlefield, c.Controller); len(bf) != 0 {
		t.Fatalf("battlefield = %v, want empty", bf)
	}
}

// TestTokenOwnerOpponentPicksTheNextAliveSeat covers TokenOwner$ Opponent in
// both a 2-player game (the only opponent) and a 4-player game (the next
// living seat after the controller, in APNAP order -- the same seat
// Defined$ Opponent's own first entry names, context_test.go's
// TestDefinedResolvesEachForm). The token must land on the OPPONENT's
// battlefield, not the controller's own.
func TestTokenOwnerOpponentPicksTheNextAliveSeat(t *testing.T) {
	for _, tc := range []struct {
		seats      int
		controller state.PlayerID
		wantOwner  state.PlayerID
	}{
		{2, 0, 1},
		{4, 1, 2},
	} {
		h := newHost(t, tc.seats)
		h.g.Tokens = tokenFixtures(t)
		c := &Ctx{Controller: tc.controller}

		Resolve(h, c, &cards.SA{Kind: "DB", API: "Token",
			Params: map[string]string{"TokenScript": "r_1_1_goblin", "TokenOwner": "Opponent"}})

		bf := h.Game().Zone(state.ZBattlefield, tc.wantOwner)
		if len(bf) != 1 {
			t.Fatalf("seats=%d controller=%d: opponent %d's battlefield = %v, want 1 token",
				tc.seats, tc.controller, tc.wantOwner, bf)
		}
		if o := h.Game().Obj(bf[0]); o.Owner != tc.wantOwner || o.Controller != tc.wantOwner {
			t.Fatalf("seats=%d controller=%d: token owner=%d controller=%d, want both %d",
				tc.seats, tc.controller, o.Owner, o.Controller, tc.wantOwner)
		}
		if own := h.Game().Zone(state.ZBattlefield, tc.controller); len(own) != 0 {
			t.Fatalf("seats=%d controller=%d: controller's own battlefield = %v, want empty",
				tc.seats, tc.controller, own)
		}
	}
}

// TestTokenOwnerDefaultsToYou: with no TokenOwner$ at all (the corpus's most
// common shape -- "You" is the default per the brief), the token belongs to
// the controller, mirroring TestTokenCreatesEachScriptTheGivenNumberOfTimes'
// own explicit "You" but proving the parameter is genuinely optional.
func TestTokenOwnerDefaultsToYou(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "r_1_1_goblin"}})
	if bf := h.Game().Zone(state.ZBattlefield, c.Controller); len(bf) != 1 {
		t.Fatalf("controller's battlefield = %v, want 1 token", bf)
	}
}

// TestTokenOwnerUnrecognizedFormNotesAndDefaultsToController: a
// TokenOwner$ this build does not model (e.g. "Defined$"-style forms
// beyond You/Opponent) still creates the token under the controller --
// the brief's own stated fallback -- but now says so with a Note, so the
// fidelity gap is visible in the log rather than silently indistinguishable
// from the ordinary "You" default.
func TestTokenOwnerUnrecognizedFormNotesAndDefaultsToController(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token",
		Params: map[string]string{"TokenScript": "r_1_1_goblin", "TokenOwner": "Player"}})

	if bf := h.Game().Zone(state.ZBattlefield, c.Controller); len(bf) != 1 {
		t.Fatalf("controller's battlefield = %v, want 1 token (unrecognised TokenOwner$ still "+
			"defaults to the controller)", bf)
	}
	want := "unrecognized TokenOwner Player, defaulting to the controller"
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && ev.Text == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("log = %+v, want a Note %q", h.log, want)
	}
}
