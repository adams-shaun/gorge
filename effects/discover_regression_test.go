package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func discoverCarrierSA(t *testing.T, reg *cards.Registry, name string) (*cards.Card, *cards.SA) {
	t.Helper()
	card, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("missing corpus card %q", name)
	}
	for _, face := range card.Faces {
		for _, ability := range face.Abilities {
			if ability.API == "Discover" {
				return card, ability
			}
		}
		if ability := cards.ResolveSVar(face.SVars, "DBDiscover"); ability != nil {
			return card, ability
		}
	}
	t.Fatalf("%s has no Discover ability", name)
	return nil, nil
}

func TestDiscoverRealCarriersOfferFreeCast(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Hidden Cataract", "Daring Discovery"} {
		t.Run(name, func(t *testing.T) {
			carrier, discover := discoverCarrierSA(t, reg, name)
			h := &askHost{}
			h.g = state.NewGame(names(2))
			source := h.g.AddObject(carrier, 0)
			landCard, _ := reg.Lookup("Forest")
			spellCard, _ := reg.Lookup("Grizzly Bears")
			land := h.g.AddObject(landCard, 0)
			found := h.g.AddObject(spellCard, 0)
			h.g.SetZone(state.ZLibrary, 0, []state.ObjID{land.ID, found.ID})
			Resolve(h, &Ctx{Source: source.ID, Controller: 0, SVars: source.Face().SVars}, discover)
			if h.asked == nil || h.asked.ResumeSA == nil || h.asked.ResumeSA.Params["WithoutManaCost"] != "True" {
				t.Fatalf("real %s did not offer a cast-free choice: %+v", name, h.asked)
			}
			if len(h.asked.Options) != 1 || h.asked.Options[0].Obj != found.ID {
				t.Fatalf("real %s options = %+v, want found card %d", name, h.asked.Options, found.ID)
			}
		})
	}
}

func TestDiscoverRealCarriersDeclineIntoHandAndMarkAction(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Hidden Cataract", "Daring Discovery"} {
		t.Run(name, func(t *testing.T) {
			carrier, discover := discoverCarrierSA(t, reg, name)
			h := newHost(t, 2)
			source := h.g.AddObject(carrier, 0)
			h.Emit(events.Event{Kind: events.MoveZone, Obj: source.ID, From: state.ZLibrary, To: state.ZBattlefield})
			landCard, ok := reg.Lookup("Forest")
			if !ok {
				t.Fatal("missing Forest")
			}
			spellCard, ok := reg.Lookup("Grizzly Bears")
			if !ok {
				t.Fatal("missing Grizzly Bears")
			}
			land := h.g.AddObject(landCard, 0)
			found := h.g.AddObject(spellCard, 0)
			h.g.SetZone(state.ZLibrary, 0, []state.ObjID{land.ID, found.ID})
			ctx := &Ctx{Source: source.ID, Controller: 0, SVars: source.Face().SVars}
			Resolve(h, ctx, discover)

			if h.g.Obj(source.ID).Zone != state.ZBattlefield {
				t.Fatalf("precondition source left battlefield: %+v", h.g.Obj(source.ID))
			}
			if h.g.Obj(found.ID).Zone != state.ZHand {
				t.Fatalf("declined discover card zone = %v, want hand", h.g.Obj(found.ID).Zone)
			}
			if h.g.Obj(land.ID).Zone != state.ZLibrary {
				t.Fatalf("preceding exiled card zone = %v, want library", h.g.Obj(land.ID).Zone)
			}
			marked := false
			for _, ev := range h.log {
				if ev.Kind == events.Discover && ev.Player == 0 && ev.Obj == source.ID {
					marked = true
				}
			}
			if !marked {
				t.Fatalf("discover action did not emit its marker: %+v", h.log)
			}
		})
	}
}
