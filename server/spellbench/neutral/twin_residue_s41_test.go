package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func s41Ref(id, name, owner, controller, zone string) protocol.ObjectRef {
	return protocol.ObjectRef{ObjectID: id, CardName: &name, OwnerSeat: owner, ControllerSeat: controller, Zone: zone}
}

// TestTwinNoForeignControllerCast pins seed 1007571457455225457, decision 85
// (run seed 41): p1 may cast the Hot Soup p0 owns and has exiled, and gorge
// offers it as a payment action. The shadow rebuilt the card in its owner's
// hand, where p1's planner cannot see it, so the neutral seat was offered a
// bare cast with no plan and cast it where the native seat, which holds the
// plan, passes. The rebuilt card must sit where the viewer's planner reads it.
func TestTwinNoForeignControllerCast(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	b := NewBuilder(reg)
	soup := s41Ref("o-soup", "Hot Soup", "p0", "p0", "exile")
	mountain := s41Ref("o-mtn", "Mountain", "p1", "p1", "battlefield")
	p1 := "p1"
	obs := &protocol.Observation{Viewer: "p1", Turn: 4, PhaseStep: "postcombat_main", ActiveSeat: &p1, PrioritySeat: &p1,
		Players: [2]protocol.PlayerObs{
			{Seat: "p0", Life: 18, Exile: []protocol.ObjectRecord{{ObjectRef: soup}}},
			{Seat: "p1", Life: 20, Battlefield: []protocol.ObjectRecord{{ObjectRef: mountain, Permanent: &protocol.Permanent{}}}},
		}}
	sh, err := b.shadow(obs, map[string]bool{soup.ObjectID: true})
	if err != nil || sh == nil {
		t.Fatalf("precondition: the shadow was not built: %v", err)
	}
	if soupOwner := sh.e.G.Obj(shadowObj(t, sh, b, soup.ObjectID)).Owner; int(soupOwner) != 1 {
		t.Fatalf("the rebuilt Hot Soup is owned by seat %d, want the viewer (seat 1)", soupOwner)
	}
	var cast bool
	for _, a := range sh.e.EnsurePaymentActions() {
		if sh.ids[a.Cast.Object] == b.IDs.Of(soup.ObjectID) && len(a.Plans) > 0 {
			cast = true
		}
	}
	if !cast {
		t.Fatal("the planner offers the viewer no plan to cast the exiled Hot Soup it may play")
	}
}

func shadowObj(t *testing.T, sh *shadow, b *Builder, id string) (obj state.ObjID) {
	t.Helper()
	for o, v := range sh.ids {
		if v == b.IDs.Of(id) {
			return o
		}
	}
	t.Fatalf("object %s is not in the shadow", id)
	return 0
}

// TestShadowEngineSkipsOpeningAsk: a Gemstone Caverns dealt into a seat's
// opening hand poses "Begin the game with it?" before the first turn. The
// rebuilt game is far past its opening and the observed board overwrites the
// first turn, so a deal that asks is reshuffled (seed 18111260664460382838,
// where every window of the game lost its payment plans).
func TestShadowEngineSkipsOpeningAsk(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	gem, ok := reg.Lookup("Gemstone Caverns")
	filler, ok2 := reg.Lookup("Plains")
	if !ok || !ok2 {
		t.Fatal("precondition: Gemstone Caverns and Plains are in the corpus")
	}
	asks := func(e *rules.Engine) bool {
		d := e.Pending()
		return d != nil && len(d.Options) > 0 && d.Options[0].Kind == "opening_yes"
	}
	b := NewBuilder(reg)
	for n := 2; n <= 12; n++ {
		var decks [2][]*cards.Card
		for i := range decks {
			for j := 0; j < shadowDeckSize; j++ {
				if j < n {
					decks[i] = append(decks[i], gem)
				} else {
					decks[i] = append(decks[i], filler)
				}
			}
		}
		first := rules.New(rules.Config{Seed: 1, Names: []string{"s0", "s1"}, Decks: [][]*cards.Card{decks[0], decks[1]}, Tokens: reg.Tokens})
		if !asks(first) {
			continue // this deal did not ask; try a deck that deals more Caverns
		}
		if e := b.shadowEngine(decks); asks(e) {
			t.Fatalf("with %d Caverns a deal still poses the opening ask: %q", n, e.Pending().Prompt)
		}
		return
	}
	t.Fatal("precondition: no deck made seed 1 pose the opening ask")
}
