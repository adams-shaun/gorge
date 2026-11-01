package observe_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// TestObservePermanentCounters pins that the v2 observation carries the
// permanent counters the engine holds, and that the neutral rebuild reads them
// back under the engine's spelling: a creature with two +1/+1 counters and a
// planeswalker with loyalty 3.
func TestObservePermanentCounters(t *testing.T) {
	reg := testcorpus.Registry(t)
	deck := catalog.Deck{CatalogID: "CounterCensus", Rows: []wire.DeckRow{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Kaya, Intangible Slayer", Count: 20}}}
	cardsInDeck, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 32)
	secret[0] = 9
	g, err := gamecfg.New(reg, secrets.NewGame(secret), [2][]*cards.Card{cardsInDeck, cardsInDeck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: 0})
	if err != nil {
		t.Fatal(err)
	}
	var bear, walker state.ObjID
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Grizzly Bears":
			if bear == 0 {
				bear = o.ID
			}
		case "Kaya, Intangible Slayer":
			if walker == 0 {
				walker = o.ID
			}
		}
	}
	if bear == 0 || walker == 0 {
		t.Fatalf("deck did not contain both objects: bear %d, planeswalker %d", bear, walker)
	}
	for _, id := range []state.ObjID{bear, walker} {
		o := g.E.G.Obj(id)
		events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
	// A keyword counter's engine spelling is title case ("Flying"), unlike the
	// upper-case P1P1/LOYALTY: the neutral rebuild must restore both.
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: bear, Counter: "Flying", Amount: 1})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: walker, Counter: "LOYALTY", Amount: 3})
	if o := g.E.G.Obj(bear); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 2 || o.Counter("Flying") != 1 {
		t.Fatalf("precondition: bear zone %v, P1P1 %d, Flying %d", o.Zone, o.Counter("P1P1"), o.Counter("Flying"))
	}
	if o := g.E.G.Obj(walker); o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 3 ||
		!strings.Contains(strings.Join(o.Face().Types, " "), "Planeswalker") {
		t.Fatalf("precondition: walker zone %v, LOYALTY %d, types %v", o.Zone, o.Counter("LOYALTY"), o.Face().Types)
	}

	tr := identity.New(g.R, g.Secret)
	if err := tr.Sync(g.R); err != nil {
		t.Fatal(err)
	}
	p := &observe.Projector{R: g.R, IDs: tr}
	for _, c := range []struct {
		id   state.ObjID
		kind string
		want uint32
	}{{bear, "p1p1", 2}, {bear, "flying", 1}, {walker, "loyalty", 3}} {
		rec, err := p.Record(0, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Permanent == nil || rec.Permanent.Counters[c.kind] != c.want {
			t.Fatalf("%v: permanent counters %+v, want %s=%d", rec.CardName, rec.Permanent, c.kind, c.want)
		}
	}
	obs, err := p.Observation(0, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	nv, err := neutral.NewBuilder(reg).View(&obs)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]map[string]int32{}
	for _, pv := range nv.Players {
		for _, cv := range pv.Battlefield {
			got[cv.Name] = cv.Counters
		}
	}
	if got["Grizzly Bears"]["P1P1"] != 2 || got["Grizzly Bears"]["Flying"] != 1 {
		t.Fatalf("neutral bear counters %v, want P1P1=2 and Flying=1", got["Grizzly Bears"])
	}
	if got["Kaya, Intangible Slayer"]["LOYALTY"] != 3 {
		t.Fatalf("neutral planeswalker counters %v, want LOYALTY=3", got["Kaya, Intangible Slayer"])
	}
	if got["Grizzly Bears"]["P1P1"] == got["Kaya, Intangible Slayer"]["LOYALTY"] {
		t.Fatal("precondition: the two counters are equal, so the assertions cannot tell them apart")
	}
}
