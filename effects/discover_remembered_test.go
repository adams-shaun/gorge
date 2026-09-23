package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestHitTheMotherLodeRemembersDiscoveredCardAcrossBothPlayOutcomes pins the
// carrier's RememberDiscovered$ rider. Its Treasure sub-ability reads the
// found card after the optional free cast has either been declined or begun.
func TestHitTheMotherLodeRemembersDiscoveredCardAcrossBothPlayOutcomes(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	carrier, discover := discoverCarrierSA(t, reg, "Hit the Mother Lode")
	foundCard, ok := reg.Lookup("Grizzly Bears")
	if !ok {
		t.Fatal("missing Grizzly Bears")
	}

	newCase := func() (*fakeHost, *Ctx, state.ObjID) {
		h := newHost(t, 2)
		h.g.Tokens = reg.Tokens
		source := h.g.AddObject(carrier, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: source.ID, From: state.ZLibrary, To: state.ZBattlefield})
		found := h.g.AddObject(foundCard, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: found.ID, From: state.ZLibrary, To: state.ZLibrary})
		return h, &Ctx{Source: source.ID, Controller: 0, SVars: source.Face().SVars}, found.ID
	}
	countTreasures := func(h *fakeHost) int {
		n := 0
		for _, ev := range h.log {
			if ev.Kind == events.TokenCreate && ev.Text == "c_a_treasure_sac" {
				n++
			}
		}
		return n
	}

	t.Run("decline", func(t *testing.T) {
		h, c, found := newCase()
		Resolve(h, c, discover)
		if h.g.Obj(found).Zone != state.ZHand {
			t.Fatalf("precondition declined card zone = %v, want hand", h.g.Obj(found).Zone)
		}
		if got := countTreasures(h); got != 8 {
			t.Fatalf("declined Discover made %d Treasures, want 8", got)
		}
	})

	t.Run("cast", func(t *testing.T) {
		h, c, found := newCase()
		asking := &askHost{fakeHost: *h}
		asking.suspendAfterAsk = true
		Resolve(asking, c, discover)
		if asking.asked == nil || asking.asked.ResumeSA == nil {
			t.Fatal("precondition Discover did not pose its free-cast ask")
		}
		if asking.g.Obj(found).Zone != state.ZExile {
			t.Fatalf("precondition found card zone = %v, want exile", asking.g.Obj(found).Zone)
		}
		asking.suspendAfterAsk = false
		asking.Emit(events.Event{Kind: events.MoveZone, Obj: found, From: state.ZExile, To: state.ZStack})
		c.Play, c.PlayDone = found, true
		Resolve(asking, c, asking.asked.ResumeSA)
		if len(c.Remembered) != 1 || c.Remembered[0].Obj != found {
			t.Fatalf("cast tail remembered = %+v, want found card %d", c.Remembered, found)
		}
		Resolve(asking, c, discover.Sub)
		if got := countTreasures(&asking.fakeHost); got != 8 {
			t.Fatalf("cast Discover made %d Treasures, want 8", got)
		}
	})
}
