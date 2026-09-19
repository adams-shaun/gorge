package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"

	"github.com/adams-shaun/gorge/state"
)

// The bare wasCastByYou family (task castprov2), pinned on real corpus cards
// in both directions:
//
//   - Nine-Lives Familiar's `K:etbCounter:REVIVAL:8:ValidCard$
//     Card.Self+wasCastByYou` gate field — the one non-ETB-trigger carrier:
//     the familiar enters with eight revival counters only when cast (the
//     gate rides the Moved-case replacement ValidCard$ read), and without
//     any when cheated in — which also deads its death-return trigger, the
//     reported symptom.
//   - Zacama, Primal Calamity's ETB trigger (`ValidCard$
//     Card.wasCastByYou+Self`): untaps all lands you control when cast,
//     nothing when cheated in. One of the 40 raw ETB carriers the same split
//     makes real.
//
// The un-cast entries are raw hand→battlefield MoveZone emits (the
// etb_counter fixture shape), so the gate must answer on its own provenance.

func TestNineLivesFamiliarCastEntersWithEightRevival(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Nine-Lives Familiar"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 1, 2
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("REVIVAL"); got != 8 {
		t.Fatalf("cast Nine-Lives Familiar entered with %d revival counters, want 8 (the wasCastByYou gate must hold)", got)
	}
}

func TestNineLivesFamiliarCheatedEntersWithoutRevival(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Nine-Lives Familiar"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	if got := e.G.Obj(id).Counter("REVIVAL"); got != 0 {
		t.Fatalf("cheated Nine-Lives Familiar entered with %d revival counters, want 0 (the wasCastByYou gate must deny)", got)
	}
}

func TestZacamaCastFromHandUntapsAllLands(t *testing.T) {
	t.Parallel()

	e := handEngine(t, corpusAlternativeCard(t, "Zacama, Primal Calamity"))
	mtn := card(t, "Name:Land\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	land := e.G.AddObject(mtn, 0)
	land.Zone = state.ZBattlefield
	land.Tapped = true
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{land.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MR] = 6, 1
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MW] = 1, 1
	castMode(t, e, id, "")
	finishCast(t, e, id)
	drainQueuedTriggers(t, e)
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("Zacama in %s, want battlefield", e.G.Obj(id).Zone)
	}
	if e.G.Obj(land.ID).Tapped {
		t.Fatal("Zacama cast from hand did not untap the land (the wasCastByYou trigger must fire)")
	}
}

func TestZacamaCheatedEntryLeavesLandsTapped(t *testing.T) {
	t.Parallel()

	e := handEngine(t, corpusAlternativeCard(t, "Zacama, Primal Calamity"))
	mtn := card(t, "Name:Land\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	land := e.G.AddObject(mtn, 0)
	land.Zone = state.ZBattlefield
	land.Tapped = true
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{land.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	drainQueuedTriggers(t, e)
	if !e.G.Obj(land.ID).Tapped {
		t.Fatal("cheated Zacama untapped the land (the wasCastByYou trigger must not fire)")
	}
}
