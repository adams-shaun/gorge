package effects

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Both spells use real compiled corpus SAs. Tokens must be minted by Apply:
// clearing Card on a creature fixture loses its type and never tests !token.
func TestTokenPredicatesOnCreatedTokens(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name                string
		tokenZone, cardZone state.Zone
	}{
		{"Hour of Reckoning", state.ZBattlefield, state.ZGraveyard},
		{"Aether Snap", state.ZExile, state.ZBattlefield},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, c := fixtureHostWithTokens(t)
			spell, ok := reg.Lookup(tc.name)
			if !ok {
				t.Fatalf("missing corpus card %q", tc.name)
			}
			source := h.g.AddObject(spell, 0).ID
			initial := h.g.Clone()
			h.Emit(events.Event{Kind: events.MoveZone, Obj: c.Source, From: state.ZLibrary, To: state.ZBattlefield})
			h.Emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
			bf := h.g.Zone(state.ZBattlefield, 0)
			if len(bf) != 2 {
				t.Fatalf("battlefield = %v", bf)
			}
			token := bf[1]
			if !h.g.Obj(token).IsToken || h.g.Obj(token).Face() == nil {
				t.Fatal("TokenCreate must supply both identity and face")
			}
			for _, spec := range []string{"Card.token", "Creature.token"} {
				if !MatchesSpec(h.g, spec, token, 0) {
					t.Errorf("%s rejects a real token", spec)
				}
				if MatchesSpec(h.g, spec, c.Source, 0) {
					t.Errorf("%s accepts a nontoken", spec)
				}
			}
			if MatchesSpec(h.g, "Creature.!token", token, 0) {
				t.Error("Creature.!token accepts a real token")
			}
			// A cardless stack ability is not a token, either.
			h.Emit(events.Event{Kind: events.AbilityPush, Obj: source, Player: 0})
			stack := h.g.Zone(state.ZStack, 0)
			if len(stack) != 1 {
				t.Fatalf("ability stack = %v", stack)
			}
			if MatchesSpec(h.g, "Card.token", stack[0], 0) {
				t.Error("Card.token accepts a cardless ability")
			}
			Resolve(h, &Ctx{Source: source, Controller: 0, SVars: spell.Faces[0].SVars}, spell.Faces[0].Abilities[0])
			if got := h.g.Obj(token).Zone; got != tc.tokenZone {
				t.Errorf("token zone = %v, want %v", got, tc.tokenZone)
			}
			if got := h.g.Obj(c.Source).Zone; got != tc.cardZone {
				t.Errorf("nontoken zone = %v, want %v", got, tc.cardZone)
			}
			for _, ev := range h.log {
				events.Apply(initial, ev)
			}
			if !reflect.DeepEqual(initial, h.g) {
				t.Fatal("event-only replay differs from resolved state")
			}
		})
	}
}
