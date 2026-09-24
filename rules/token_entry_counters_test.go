package rules

import (
	"maps"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TokenCreate and CardToken enter during their mint event, without a separate
// MoveZone. Their intrinsic counters must use the same atomic entry payload
// and AddCounter replacement path as an ordinary card's MoveZone.
func TestTokenEntryCountersUseReplacements(t *testing.T) {
	for _, mint := range []events.Kind{events.TokenCreate, events.CardToken} {
		for _, tc := range []struct {
			rider, card, counter string
			base, want           int32
		}{
			{"Vorinclex, Monstrous Raider", "Jace, the Mind Sculptor", "LOYALTY", 3, 6},
			{"Solemnity", "Urza's Saga", "LORE", 1, 0},
		} {
			t.Run(tc.rider+"/"+mint.String(), func(t *testing.T) {
				entrant := tokenReplCorpusCard(t, tc.card)
				if grants := events.EntryCounterGrants(&state.Object{Card: entrant}, false); len(grants) != 1 || grants[0].Kind != tc.counter || grants[0].Amount != tc.base || tc.base == tc.want {
					t.Fatalf("precondition: %s entry grants = %v, want %d %s (different from %d)", tc.card, grants, tc.base, tc.counter, tc.want)
				}
				// An unreplaced mint must actually enter with this counter;
				// in particular a blocked Saga lore counter cannot pass just
				// because tokens never received lore in the first place.
				base, baseCfg := tokenReplGame(t, 195, entrant)
				baseCfg.Tokens = maps.Clone(baseCfg.Tokens)
				baseCfg.Tokens["test_token_entry"] = entrant
				base = New(baseCfg)
				base.Advance()
				baseEvent := events.Event{Kind: mint, Player: 0, Text: "test_token_entry"}
				if mint == events.CardToken {
					baseEvent.Obj = seededEntryID(t, base, entrant)
				}
				baseID := base.G.NextID
				base.emit(baseEvent)
				if o := base.G.Obj(baseID); o == nil || o.Zone != state.ZBattlefield {
					t.Fatalf("precondition: unreplaced token did not enter battlefield: %v", o)
				} else if got := o.Counter(tc.counter); got != tc.base {
					t.Fatalf("precondition: unreplaced token has %d %s, want %d", got, tc.counter, tc.base)
				}
				replayCheck(t, base, baseCfg)

				rider := tokenReplCorpusCard(t, tc.rider)
				e, cfg := tokenReplGame(t, 196, rider, entrant)
				// TokenCreate reads a token script from the match's pinned
				// registry; use the real corpus card as its definition. CardToken
				// instead copies the seeded card's printed face from its hand.
				cfg.Tokens = maps.Clone(cfg.Tokens)
				cfg.Tokens["test_token_entry"] = entrant
				e = New(cfg)
				e.Advance()
				rid := moveSeededCard(t, e, 0, rider, state.ZBattlefield)
				if o := e.G.Obj(rid); o == nil || o.Zone != state.ZBattlefield {
					t.Fatal("precondition: replacement source is not on the battlefield")
				}
				ev := events.Event{Kind: mint, Player: 0, Text: "test_token_entry"}
				if mint == events.CardToken {
					ev.Obj = seededEntryID(t, e, entrant)
					if o := e.G.Obj(ev.Obj); o == nil || o.Zone == state.ZBattlefield {
						t.Fatal("precondition: CardToken's source card already entered")
					}
				}
				e.SetCounterAdder(0)
				id := e.G.NextID
				e.emit(ev)
				o := e.G.Obj(id)
				if o == nil || !o.IsToken || o.Zone != state.ZBattlefield {
					t.Fatalf("token did not enter the battlefield: %+v", o)
				}
				if got := o.Counter(tc.counter); got != tc.want {
					t.Fatalf("entry %s = %d, want %d (unreplaced %d)", tc.counter, got, tc.want, tc.base)
				}
				entry := false
				for _, logged := range e.L.Events {
					if logged.Kind == mint && logged.Player == 0 &&
						(mint == events.TokenCreate && logged.Text == ev.Text || mint == events.CardToken && logged.Obj == ev.Obj) {
						entry = true
						if tc.want > 0 && len(logged.Pairs) == 0 {
							t.Fatal("entry lacks atomic counter payload")
						}
					}
				}
				if !entry {
					t.Fatal("precondition: entry event not logged")
				}
				replayCheck(t, e, cfg)
			})
		}
	}
}
