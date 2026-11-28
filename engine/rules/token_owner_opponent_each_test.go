package rules

// CR 111.2 / CR 101.4: "each opponent creates ..." scripted `TokenOwner$
// Opponent` (Captive Audience, Eiganjo Uprising, Pursued Whale, ...) makes
// every living opponent create its own tokens, while a targeted spelling
// (Questing Phelddagrif's `ValidTgts$ Opponent | TokenOwner$ Opponent`,
// "target opponent creates") gives them to the chosen opponent only.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const tokenOwnerOpponentFixture = "Name:Test Opponent Token Maker\nTypes:Artifact\n" +
	"SVar:Each:DB$ Token | TokenAmount$ 2 | TokenScript$ r_1_1_goblin | TokenOwner$ Opponent\n" +
	"SVar:Tgt:DB$ Token | ValidTgts$ Opponent | TokenScript$ r_1_1_goblin | TokenOwner$ Opponent\n" +
	"Oracle:x\n"

const tokenOwnerOpponentGoblin = "Name:Goblin Token\nTypes:Creature Goblin\nColors:red\nPT:1/1\nOracle:\n"

func inlineTokenCard(t *testing.T, name, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes(name, []byte(src))
	if len(diags) != 0 {
		t.Fatalf("fixture parse %s: %v", name, diags)
	}
	c.Link()
	return c
}

// tokenOwnerOpponentGame builds a four-seat engine with the maker on seat 1's
// battlefield (seat 0 is active), and seat 3 already lost when dead is set.
func tokenOwnerOpponentGame(t *testing.T, dead bool) (*Engine, state.ObjID, *cards.Card) {
	t.Helper()
	maker := inlineTokenCard(t, "inline-maker.txt", tokenOwnerOpponentFixture)
	goblin := inlineTokenCard(t, "r_1_1_goblin.txt", tokenOwnerOpponentGoblin)
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = maker
	}
	e := New(seatZeroStart(Config{Seed: 11, Names: []string{"a", "b", "c", "d"},
		Decks:  [][]*cards.Card{deck, deck, deck, deck},
		Tokens: map[string]*cards.Card{"r_1_1_goblin": goblin}}))
	if len(e.G.Players) != 4 || e.G.Active != 0 {
		t.Fatalf("precondition: %d seats, active %d; want 4 seats, seat 0 active", len(e.G.Players), e.G.Active)
	}
	if dead {
		e.G.Players[3].Lost = true
	}
	id := e.G.Zone(state.ZLibrary, 1)[0]
	e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield, Player: 1})
	return e, id, maker
}

func goblinTokensOf(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Owner == p && o.Controller == p {
			n++
		}
	}
	return n
}

func TestTokenOwnerOpponentEachOpponent(t *testing.T) {
	t.Parallel()
	t.Run("each living opponent", func(t *testing.T) {
		e, src, maker := tokenOwnerOpponentGame(t, true)
		body := cards.ResolveSVar(maker.Faces[0].SVars, "Each")
		if body == nil || body.API != "Token" {
			t.Fatalf("precondition: missing Token body: %+v", body)
		}
		effects.Resolve(e, effects.NewCtxPtr(src, 1, effects.CtxInit{}), body)
		want := []int{2, 0, 2, 0}
		for p, w := range want {
			if got := goblinTokensOf(e, state.PlayerID(p)); got != w {
				t.Errorf("seat %d created %d Goblins, want %d", p, got, w)
			}
		}
	})
	t.Run("target opponent only", func(t *testing.T) {
		e, src, maker := tokenOwnerOpponentGame(t, false)
		body := cards.ResolveSVar(maker.Faces[0].SVars, "Tgt")
		if body == nil || body.API != "Token" {
			t.Fatalf("precondition: missing Token body: %+v", body)
		}
		// The activation's target ask chose seat 3 (the placement ask covered
		// this SA, so resolution does not ask again).
		ctx := effects.NewCtxPtr(src, 1, effects.CtxInit{Targets: []state.Target{{IsPlayer: true, Player: 3}}})
		ctx.OfferedSA = body
		ctx.TargetsOffered = true
		effects.Resolve(e, ctx, body)
		want := []int{0, 0, 0, 1}
		for p, w := range want {
			if got := goblinTokensOf(e, state.PlayerID(p)); got != w {
				t.Errorf("seat %d created %d Goblins, want %d", p, got, w)
			}
		}
	})
}
