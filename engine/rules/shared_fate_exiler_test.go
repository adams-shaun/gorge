package rules

// Shared Fate: "If a player would draw a card, that player exiles the top
// card of one of their opponents' libraries face down instead. Each player
// may look at cards they exiled with Shared Fate, and they may play lands
// and cast spells from among those cards." The printed static's
// MayPlayPlayer$ Exiler names the player who exiled the card (the Draw
// replacement's ReplacedPlayer), not the static's controller or the card's
// owner.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sharedFateExile puts Shared Fate on seat 0's battlefield and name on top of
// victim's library, has drawer draw (the replacement asks drawer to choose
// victim) and returns the exiled card's id, Shared Fate's id and the exile
// event.
func sharedFateExile(t *testing.T, e *Engine, drawer, victim state.PlayerID, name string) (mid, sfID state.ObjID, ev events.Event) {
	t.Helper()
	sfID = moveByName(t, e, 0, "Shared Fate", state.ZBattlefield)
	mid = moveByName(t, e, victim, name, state.ZLibrary)
	order := []state.ObjID{mid}
	for _, id := range e.G.Zone(state.ZLibrary, victim) {
		if id != mid {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: victim, IDs: order})
	mark := len(e.L.Events)
	emitDraw(t, e, drawer)
	// With one opponent the ChoosePlayer ask has a single option and is
	// answered without a decision; with more it asks the drawer.
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		if d.Player != drawer {
			t.Fatalf("ChoosePlayer ask addressed to seat %d, want the drawer %d", d.Player, drawer)
		}
		pick := indexOfPlayerOption(d, victim)
		if pick < 0 {
			t.Fatalf("seat %d is not offered as the opponent: %+v", victim, d.Options)
		}
		submitChoices(t, e, pick)
	}
	if e.Pending() == nil {
		e.priorityRound()
	}
	ev = faceDownExileMoveOf(t, e, mark, mid)
	return mid, sfID, ev
}

func assertExilerRecorded(t *testing.T, e *Engine, mid, sfID state.ObjID, exiler state.PlayerID) {
	t.Helper()
	o := e.G.Obj(mid)
	if o == nil || o.Zone != state.ZExile || !o.FaceDown {
		t.Fatalf("precondition: %d is not face down in exile: %+v", mid, o)
	}
	if !o.HasExiledBy || o.ExiledBy != exiler || o.ExiledWith != sfID {
		t.Fatalf("exile provenance = (has %v, by %d, with %d), want (true, %d, %d)", o.HasExiledBy, o.ExiledBy, o.ExiledWith, exiler, sfID)
	}
}

func TestSharedFateExilerMayCastFromExile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Shared Fate")}, []*cards.Card{lookup(t, reg, "Elvish Mystic")})
	mid, sfID, ev := sharedFateExile(t, e, 0, 1, "Elvish Mystic")
	assertExilerRecorded(t, e, mid, sfID, 0)
	only0 := map[state.PlayerID]bool{0: true}
	assertFaceDownExileLookers(t, e, ev, "Elvish Mystic", only0, only0)
	if !e.MayLookAtFaceDown(0, mid) || e.MayLookAtFaceDown(1, mid) {
		t.Errorf("MayLookAtFaceDown: seat 0 = %v, seat 1 = %v; want true, false", e.MayLookAtFaceDown(0, mid), e.MayLookAtFaceDown(1, mid))
	}
	if _, ok := e.mayPlayGrant(0, mid); !ok {
		t.Error("the exiler (seat 0) has no may-play grant")
	}
	if _, ok := e.mayPlayGrant(1, mid); ok {
		t.Error("the owner (seat 1) has a may-play grant")
	}
	has := func(offers []mayPlaySpellOffer) bool {
		for _, of := range offers {
			if of.id == mid {
				return true
			}
		}
		return false
	}
	if !has(e.mayPlaySpellIds(0)) || has(e.mayPlaySpellIds(1)) {
		t.Error("mayPlaySpellIds must offer the exiled card to seat 0 and not to seat 1")
	}
	addMana(t, e, 0, "G")
	if !hasCastOptionFor(e, mid) {
		t.Fatalf("no cast option for the exiled card: %+v", e.Pending())
	}
	e.beginCast(0, castOptionFor(t, e, mid))
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(mid)
	if o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("cast card: zone %v controller %d, want battlefield under seat 0", o.Zone, o.Controller)
	}
	replayCheck(t, e, cfg)
}

func TestSharedFateExilerMayPlayLandFromExile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Shared Fate")}, []*cards.Card{lookup(t, reg, "Forest")})
	fid, sfID, _ := sharedFateExile(t, e, 0, 1, "Forest")
	assertExilerRecorded(t, e, fid, sfID, 0)
	if !e.mayPlayLandGranted(0, fid) || e.mayPlayLandGranted(1, fid) {
		t.Fatalf("mayPlayLandGranted: seat 0 = %v, seat 1 = %v; want true, false", e.mayPlayLandGranted(0, fid), e.mayPlayLandGranted(1, fid))
	}
	if !e.mayPlayLandAny(0) {
		t.Error("mayPlayLandAny(0) disagrees with mayPlayLandIds")
	}
	e.priorityRound()
	if !playOneLand(t, e, 0, fid) {
		t.Fatal("the exiled Forest was not offered as a land play to seat 0")
	}
	o := e.G.Obj(fid)
	if o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("played land: zone %v controller %d, want battlefield under seat 0", o.Zone, o.Controller)
	}
	replayCheck(t, e, cfg)
}

func TestSharedFateExilerIsTheDrawerNotTheController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Shared Fate"), lookup(t, reg, "Elvish Mystic")}, nil)
	mid, sfID, ev := sharedFateExile(t, e, 1, 0, "Elvish Mystic")
	assertExilerRecorded(t, e, mid, sfID, 1)
	only1 := map[state.PlayerID]bool{1: true}
	assertFaceDownExileLookers(t, e, ev, "Elvish Mystic", only1, only1)
	if !e.MayLookAtFaceDown(1, mid) || e.MayLookAtFaceDown(0, mid) {
		t.Errorf("MayLookAtFaceDown: seat 1 = %v, seat 0 = %v; want true, false", e.MayLookAtFaceDown(1, mid), e.MayLookAtFaceDown(0, mid))
	}
	if _, ok := e.mayPlayGrant(1, mid); !ok {
		t.Error("the exiler (seat 1) has no may-play grant")
	}
	if _, ok := e.mayPlayGrant(0, mid); ok {
		t.Error("the controller and owner (seat 0) has a may-play grant")
	}
	if len(e.mayPlayPermissions(1, mid, true)) == 0 || len(e.mayPlayPermissions(0, mid, true)) != 0 {
		t.Errorf("mayPlayPermissions: seat 1 = %d, seat 0 = %d; want >0, 0", len(e.mayPlayPermissions(1, mid, true)), len(e.mayPlayPermissions(0, mid, true)))
	}
	replayCheck(t, e, cfg)
}

func TestMayPlayGateRejectedOtherExilerOnly(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]bool{"Exiler": false, "": false, "CardOwner": true, "ActivePlayer": true, "Player": true, "Player.Active": true} {
		params := map[string]string{}
		if v != "" {
			params["MayPlayPlayer"] = v
		}
		if got := mayPlayGateRejectedOther(params); got != want {
			t.Errorf("mayPlayGateRejectedOther(MayPlayPlayer=%q) = %v, want %v", v, got, want)
		}
	}
}
