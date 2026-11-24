package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR70124ObjectPathEmptySetShufflesNamedLibrary pins CR 701.24d: "If an
// effect would cause a player to shuffle a set of objects into a library,
// that library is shuffled even if there are no objects in that set." An
// object-target ChangeZone's Shuffle$ True tail must still name the library
// the effect independently names when the target ask answered zero objects:
// the parent player of TargetsWithDefinedController$ ParentTarget, or the
// controller whose cards a YouCtrl/YouOwn target filter restricts to. A
// filter that names only the moved cards' owners (Turn the Earth's Card)
// must NOT gain a controller fallback: with zero objects it names no
// library.
func TestCR70124ObjectPathEmptySetShufflesNamedLibrary(t *testing.T) {
	t.Run("parent player", func(t *testing.T) {
		ah := &fx42AskHost{}
		ah.g = state.NewGame(names(2))
		src := ah.g.AddObject(creature(t, "Avatar"), 0)
		c := &Ctx{Source: src.ID, Controller: 0,
			parentLinks: [][]state.Target{{{IsPlayer: true, Player: 1}}}, TargetsOffered: true}
		s := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | TargetMin$ 0 | TargetMax$ 4 | TargetsWithDefinedController$ ParentTarget | Shuffle$ True | ValidTgts$ Card")
		effChangeZone(ah, c, s)
		if got := shuffleEvents(&ah.fakeHost, 1); got != 1 {
			t.Fatalf("parent-target player's shuffles = %d, want exactly 1 (CR 701.24d): %+v", got, ah.log)
		}
		if got := shuffleEvents(&ah.fakeHost, 0); got != 0 {
			t.Fatalf("controller's shuffles = %d, want 0 (the effect names the parent player's library)", got)
		}
	})

	t.Run("your library", func(t *testing.T) {
		ah := &fx42AskHost{}
		ah.g = state.NewGame(names(2))
		src := ah.g.AddObject(creature(t, "Avatar"), 0)
		c := &Ctx{Source: src.ID, Controller: 0, TargetsOffered: true}
		s := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | TargetMin$ 0 | TargetMax$ X | Shuffle$ True | ValidTgts$ Creature.YouCtrl")
		effChangeZone(ah, c, s)
		if got := shuffleEvents(&ah.fakeHost, 0); got != 1 {
			t.Fatalf("controller's shuffles = %d, want exactly 1 (the YouCtrl filter names your library): %+v", got, ah.log)
		}
		if got := shuffleEvents(&ah.fakeHost, 1); got != 0 {
			t.Fatalf("other seat's shuffles = %d, want 0", got)
		}
	})

	t.Run("owner-defined guard", func(t *testing.T) {
		ah := &fx42AskHost{}
		ah.g = state.NewGame(names(2))
		src := ah.g.AddObject(creature(t, "Avatar"), 0)
		c := &Ctx{Source: src.ID, Controller: 0, TargetsOffered: true}
		s := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | TargetMin$ 0 | TargetMax$ 3 | Shuffle$ True | ValidTgts$ Card")
		effChangeZone(ah, c, s)
		for p := range 2 {
			if got := shuffleEvents(&ah.fakeHost, state.PlayerID(p)); got != 0 {
				t.Fatalf("seat %d's shuffles = %d, want 0: with zero objects the owner-defined set names no library (Turn the Earth)", p, got)
			}
		}
	})

	t.Run("nonempty control", func(t *testing.T) {
		ah := &fx42AskHost{}
		ah.g = state.NewGame(names(2))
		src := ah.g.AddObject(creature(t, "Avatar"), 0)
		gy := ah.g.AddObject(creature(t, "Grizzly Bears"), 1)
		gy.Zone = state.ZGraveyard
		ah.g.SetZone(state.ZGraveyard, 1, []state.ObjID{gy.ID})
		c := &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{Obj: gy.ID}},
			parentLinks: [][]state.Target{{{IsPlayer: true, Player: 1}}}, TargetsOffered: true}
		s := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | TargetMin$ 0 | TargetMax$ 4 | TargetsWithDefinedController$ ParentTarget | Shuffle$ True | ValidTgts$ Card")
		effChangeZone(ah, c, s)
		if o := ah.g.Obj(gy.ID); o == nil || o.Zone != state.ZLibrary {
			t.Fatalf("precondition: moved card zone = %+v, want library", o)
		}
		if got := shuffleEvents(&ah.fakeHost, 1); got != 1 {
			t.Fatalf("parent-target player's shuffles = %d, want exactly 1 (owner and implicit parent must deduplicate): %+v", got, ah.log)
		}
	})

	t.Run("your library owner is not the controller", func(t *testing.T) {
		ah := &fx42AskHost{}
		ah.g = state.NewGame(names(2))
		src := ah.g.AddObject(creature(t, "Avatar"), 0)
		stolen := ah.g.AddObject(creature(t, "Grizzly Bears"), 0) // controller 0, owner 1
		o := ah.g.Obj(stolen.ID)
		o.Owner, o.Controller, o.Zone = 1, 0, state.ZBattlefield
		ah.g.SetZone(state.ZBattlefield, 0, []state.ObjID{stolen.ID})
		c := &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{Obj: stolen.ID}}, TargetsOffered: true}
		s := sa(t, "DB$ ChangeZone | Origin$ Battlefield | Destination$ Library | TargetMin$ 0 | TargetMax$ 1 | Shuffle$ True | ValidTgts$ Creature.YouCtrl")
		effChangeZone(ah, c, s)
		if got := shuffleEvents(&ah.fakeHost, 1); got != 1 {
			t.Fatalf("owner's shuffles = %d, want exactly 1: the moved card's owner's library is the one shuffled", got)
		}
		if got := shuffleEvents(&ah.fakeHost, 0); got != 0 {
			t.Fatalf("controller's shuffles = %d, want 0: a YouCtrl filter that chose another player's card names the owner's library, not the controller's too", got)
		}
	})
}
