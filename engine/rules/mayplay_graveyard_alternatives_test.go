package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Kagha's and Coram's may-play statics, verbatim from the Forge scripts the
// corpus carries (inline: scripts are never committed).
const kaghaGraveSrc = "Name:Kagha, Shadow Archdruid\nManaCost:2 G G\nTypes:Legendary Creature Elf Druid\nPT:4/4\n" +
	"S:Mode$ Continuous | Affected$ Land.YouOwn+ThisTurnEnteredFrom_Library,Permanent.YouOwn+ThisTurnEnteredFrom_Library | Condition$ PlayerTurn | MayPlay$ True | MayPlayLimit$ 1 | EffectZone$ Battlefield | AffectedZone$ Graveyard | Description$ x\n" +
	"Oracle:x\n"

const coramGraveSrc = "Name:Coram, the Undertaker\nManaCost:2 B G\nTypes:Legendary Creature Human Warrior\nPT:3/3\n" +
	"S:Mode$ Continuous | Affected$ Land.ThisTurnEnteredFrom_Library | Condition$ PlayerTurn | MayPlay$ True | MayPlayLimit$ 1 | MayPlayText$ Land | EffectZone$ Battlefield | AffectedZone$ Graveyard | Description$ x\n" +
	"Oracle:x\n"

// milledFromLibrary puts a fresh card owned by owner on top of its library and
// mills it through the MoveZone event the replay folds, so the object carries
// the ThisTurnEnteredFrom_Library provenance the statics read.
func milledFromLibrary(t *testing.T, e *Engine, src string, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), owner)
	o.Zone = state.ZLibrary
	ids := append([]state.ObjID{o.ID}, e.G.Zone(state.ZLibrary, owner)...)
	e.G.SetZone(state.ZLibrary, owner, ids)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZGraveyard})
	if got := e.G.Obj(o.ID); got.Zone != state.ZGraveyard || !got.EnteredThisTurn || got.EnteredFrom != state.ZLibrary {
		t.Fatalf("precondition: milled card zone=%v enteredThisTurn=%v from=%v", got.Zone, got.EnteredThisTurn, got.EnteredFrom)
	}
	return o.ID
}

// TestKaghaGraveyardPermanentCast pins the second comma alternative of
// Kagha's Affected$: a creature card milled from its own library this turn is
// castable from the graveyard (Permanent means a permanent CARD there), while
// one milled from an opponent's library (YouOwn) and one put into the
// graveyard from hand (ThisTurnEnteredFrom_Library) are not.
func TestKaghaGraveyardPermanentCast(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	onBoardGrant(t, e, 0, kaghaGraveSrc)
	e.G.Players[0].Pool[state.MG] = 6
	milled := milledFromLibrary(t, e, creatureSrc("Milled Bear"), 0)
	theirs := milledFromLibrary(t, e, creatureSrc("Their Bear"), 1)
	fromHand := graveCard(e, card(t, creatureSrc("Hand Bear")), 0, 0)

	if len(e.mayPlayOffersFor(milled)) == 0 {
		t.Fatalf("own milled creature not offered: %+v", e.legalActions(0))
	}
	if n := len(e.mayPlayOffersFor(theirs)); n != 0 {
		t.Errorf("opponent's milled creature offered %d times, want 0", n)
	}
	if n := len(e.mayPlayOffersFor(fromHand)); n != 0 {
		t.Errorf("creature that did not come from the library offered %d times, want 0", n)
	}
}

// TestCoramOpponentGraveyardLand pins Coram's "play a land ... from among
// cards in graveyards": a land milled from an OPPONENT's library this turn is
// offered as a land play and playing it works, while an opponent's land that
// reached the graveyard another way is not.
func TestCoramOpponentGraveyardLand(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	onBoardGrant(t, e, 0, coramGraveSrc)
	mine := milledFromLibrary(t, e, landSrc("Own Land"), 0)
	theirs := milledFromLibrary(t, e, landSrc("Their Land"), 1)
	other := graveCard(e, card(t, landSrc("Discarded Land")), 1, 1)

	if countPlayLand(e, mine) != 1 {
		t.Fatalf("own milled land not offered: %+v", e.legalActions(0))
	}
	if countPlayLand(e, theirs) != 1 {
		t.Fatalf("opponent's milled land not offered: %+v", e.legalActions(0))
	}
	if n := countPlayLand(e, other); n != 0 {
		t.Errorf("opponent's non-milled land offered %d times, want 0", n)
	}
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == theirs {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no play_land option for the opponent's milled land: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}
	if o := e.G.Obj(theirs); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("played land zone=%v controller=%v, want seat 0's battlefield", o.Zone, o.Controller)
	}
}
