package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "Attached" reads relative to the source (Forge's source.hasCardAttachment; read): Zack
// Fair's "an Equipment that was attached to Zack Fair" and Myr Adapter's "for each
// Equipment attached to this creature" do not reach an Equipment on another creature.
func attachedSourceBoard(t *testing.T, name string) (*Engine, Config, func(string, state.PlayerID) state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	seat := func() []*cards.Card {
		return append([]*cards.Card{mustCorpusCard(t, reg, name), mustCorpusCard(t, reg, "Grizzly Bears"), mustCorpusCard(t, reg, "Bonesplitter")}, mountainDeck(t, 37)...)
	}
	cfg := seatZeroStart(Config{Seed: 3, Names: []string{"a", "b"}, Decks: [][]*cards.Card{seat(), seat()}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg, func(card string, p state.PlayerID) state.ObjID {
		id := findByName(e, card, p)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
		return id
	}
}

func TestZackFairMovesOnlyWhatWasAttachedToHim(t *testing.T) {
	t.Parallel()
	for _, equipped := range []bool{false, true} {
		e, cfg, put := attachedSourceBoard(t, "Zack Fair")
		zack, mine := put("Zack Fair", 0), put("Grizzly Bears", 0)
		theirs, glove := put("Grizzly Bears", 1), put("Bonesplitter", 1)
		e.emit(events.Event{Kind: events.Attach, Obj: glove, IDs: []state.ObjID{theirs}})
		var own state.ObjID
		if equipped {
			own = put("Bonesplitter", 0)
			e.emit(events.Event{Kind: events.Attach, Obj: own, IDs: []state.ObjID{zack}})
		}
		e.pending = nil
		e.priorityRound()
		addMana(t, e, 0, "C")
		submitChoices(t, e, abilityOption(t, e, zack, 0).Index)
		chooseTargetObject(t, e, mine)
		passUntilStackEmpty(t, e, 20)
		if z := e.G.Obj(zack).Zone; z != state.ZGraveyard {
			t.Fatalf("Zack in %v, want the graveyard", z)
		}
		if got := e.G.Obj(glove).AttachedTo; got != theirs {
			t.Fatalf("equipped=%v: the opponent's Equipment is on %d, want %d", equipped, got, theirs)
		}
		if equipped && e.G.Obj(own).AttachedTo != mine {
			t.Fatalf("Zack's Equipment is on %d, want the target %d", e.G.Obj(own).AttachedTo, mine)
		}
		replayCheck(t, e, cfg)
	}
}

func TestMyrAdapterCountsOnlyItsOwnEquipment(t *testing.T) {
	t.Parallel()
	e, cfg, put := attachedSourceBoard(t, "Myr Adapter")
	myr, bear := put("Myr Adapter", 0), put("Grizzly Bears", 1)
	e.emit(events.Event{Kind: events.Attach, Obj: put("Bonesplitter", 1), IDs: []state.ObjID{bear}})
	if p := e.Power(myr); p != 1 {
		t.Fatalf("power %d with an Equipment on another creature, want 1", p)
	}
	own := put("Bonesplitter", 0)
	e.emit(events.Event{Kind: events.Attach, Obj: own, IDs: []state.ObjID{myr}})
	if p, tg := e.Power(myr), e.Toughness(myr); p != 4 || tg != 2 {
		t.Fatalf("%d/%d with Bonesplitter attached, want 4/2", p, tg)
	}
	replayCheck(t, e, cfg)
}
