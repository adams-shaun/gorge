package observe_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// TestObservePoisonUsesTheDedicatedField pins that the engine's "POISON" player
// counter projects into the poison field and not into counters, and that a
// zero-valued poison counter keeps enough presence in counters for the neutral
// rebuild to preserve native's POISON:0 entry.
func TestObservePoisonUsesTheDedicatedField(t *testing.T) {
	reg := testcorpus.Registry(t)
	deck := catalog.Deck{CatalogID: "PoisonProbe", Rows: []wire.DeckRow{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Kaya, Intangible Slayer", Count: 20}}}
	cardsInDeck, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 32)
	secret[0] = 11
	g, err := gamecfg.New(reg, secrets.NewGame(secret), [2][]*cards.Card{cardsInDeck, cardsInDeck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: 0})
	if err != nil {
		t.Fatal(err)
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 3})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: 2})
	if got := g.E.G.Players[0].Counter("POISON"); got != 3 {
		t.Fatalf("precondition: poison %d", got)
	}
	if got := g.E.G.Players[0].Counter("Poison"); got != 0 {
		t.Fatalf("precondition: the engine uses the POISON spelling, not Poison: %d", got)
	}

	tr := identity.New(g.R, g.Secret)
	if err := tr.Sync(g.R); err != nil {
		t.Fatal(err)
	}
	p := &observe.Projector{R: g.R, IDs: tr}
	obs, err := p.Observation(0, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	po := obs.Players[0]
	if po.Poison == nil || *po.Poison != 3 {
		t.Fatalf("poison field %v, want 3", po.Poison)
	}
	if n, ok := po.Counters["poison"]; ok {
		t.Fatalf("positive poison is double-routed into counters: %d", n)
	}
	if po.Counters["energy"] != 2 {
		t.Fatalf("energy counter %d, want 2", po.Counters["energy"])
	}
	nv, err := neutral.NewBuilder(reg).View(&obs)
	if err != nil {
		t.Fatal(err)
	}
	if got := nv.Players[0].Counters["POISON"]; got != 3 {
		t.Fatalf("neutral POISON %d, want 3", got)
	}

	// Removing the counters leaves zero-valued entries in state. The poison
	// field cannot express their presence, so they must also ride counters;
	// otherwise the neutral rebuild silently drops native's POISON:0.
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: -3})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: -2})
	if got := g.E.G.Players[0].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: poison %d after removal", got)
	}
	obs, err = p.Observation(0, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	po = obs.Players[0]
	if po.Poison == nil || *po.Poison != 0 {
		t.Fatalf("zero poison field %v, want a present 0", po.Poison)
	}
	if n, ok := po.Counters["poison"]; !ok || n != 0 {
		t.Fatalf("zero poison presence in counters = %d, present %v; want 0, true", n, ok)
	}
	nv, err = neutral.NewBuilder(reg).View(&obs)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := nv.Players[0].Counters["POISON"]; !ok || got != 0 {
		t.Fatalf("neutral zero POISON = %d, present %v; want 0, true", got, ok)
	}
}
