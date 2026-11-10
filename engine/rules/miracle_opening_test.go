package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The cards dealt into an opening hand are drawn, but they are not "the first
// card you've drawn this turn": no turn has begun (CR 103.5, 702.94a). A miracle
// card on top of the shuffled library gets no offer before turn 1.
func TestMiracleNotOfferedOnTheOpeningHand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	terminus := lookup(t, reg, "Terminus")
	deck := make([]*cards.Card, 60)
	for i := range deck {
		deck[i] = terminus
	}
	for _, mulligans := range []int{0, 2} {
		cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"}, Tokens: reg.Tokens,
			Decks: [][]*cards.Card{deck, deck}, Mulligans: mulligans})
		e := New(cfg)
		e.Advance()
		for d := e.Pending(); d != nil && e.G.Turn <= 1; d = e.Pending() {
			if d.Kind == decision.KTriggerOptional {
				t.Fatalf("mulligans %d: miracle offered at turn %d, before any turn's draw: %q", mulligans, e.G.Turn, d.Prompt)
			}
			if d.Kind == decision.KPriority {
				passOnce(t, e)
			} else {
				submitChoices(t, e, d.Options[0].Index)
			}
		}
	}
}

// A miracle granted to cards in hand offers and charges like a printed one: Lorehold,
// the Historian's "miracle {2}", and Aminatou, Veil Piercer's cost, the card's own
// less {4} (CR 702.94a; XMage's grant read, not run).
func TestMiracleGrantedInHandOffersAndCharges(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		granter, spell string
		wantPool       int32
	}{
		{"Lorehold, the Historian", "Name:Big Sorcery\nManaCost:9\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n", 4},
		{"Aminatou, Veil Piercer", "Name:Big Aura\nManaCost:6 W\nTypes:Enchantment\nOracle:x\n", 3},
	} {
		e, _ := animateTriggersEngine(t, reg, lookup(t, reg, tc.granter))
		moveByName(t, e, 0, tc.granter, state.ZBattlefield)
		id := onBoard(t, e, 0, tc.spell)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZLibrary})
		addMana(t, e, 0, "WWWWWW")
		e.pendingTriggers = nil
		e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: id, From: state.ZLibrary, To: state.ZHand, Secret: true})
		if len(e.pendingTriggers) != 1 || !e.pendingTriggers[0].Miracle {
			t.Fatalf("%s: no miracle offer: %+v", tc.granter, e.pendingTriggers)
		}
		e.pending = nil
		e.Advance()
		d := e.Pending()
		if d == nil || d.Kind != decision.KTriggerOptional {
			t.Fatalf("%s: offer decision %+v", tc.granter, d)
		}
		submitChoices(t, e, 0) // reveal: yes
		miracleTriggerOnStack(t, e, id)
		ask := passToMiracleAsk(t, e)
		if ask == nil {
			t.Fatalf("%s: no miracle cast ask after the trigger resolved", tc.granter)
		}
		submitChoices(t, e, miracleOption(t, ask, "yes"))
		if e.G.Obj(id).Zone != state.ZStack {
			t.Fatalf("%s: zone %v, want the stack", tc.granter, e.G.Obj(id).Zone)
		}
		if got := e.G.Players[0].Pool.Total(); int32(got) != tc.wantPool {
			t.Fatalf("%s: pool %d after the cast, want %d", tc.granter, got, tc.wantPool)
		}
	}
}
