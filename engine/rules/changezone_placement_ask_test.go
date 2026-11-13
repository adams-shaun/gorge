package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// pendingArrange is the pending KArrange the test is parked on, failing with
// the actual pending decision when there is none.
func pendingArrange(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("pending = %+v, want the CR 401.4 placement KArrange", d)
	}
	if len(d.Options) < 2 || d.Options[0].Kind != "library_placement" || d.Min != len(d.Options) || d.Max != len(d.Options) {
		t.Fatalf("arrange = %+v, want a library_placement Min==Max ask over >= 2 cards", d)
	}
	return d
}

// TestTerminusBottomsCreaturesInAnsweredOrder: CR 401.4 -- Terminus' two
// seat-0 creatures go to the bottom of their owner's library in the order
// the owner answers, a non-offered order, and the game replays.
func TestTerminusBottomsCreaturesInAnsweredOrder(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	terminus := mustCorpusCard(t, reg, "Terminus")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	elves := mustCorpusCard(t, reg, "Llanowar Elves")
	e, cfg := censusEngine(t, 4712, []*cards.Card{terminus, bear, elves}, []*cards.Card{bear})
	cardToHand(t, e, terminus)
	bear0 := moveOwnerCard(t, e, 0, bear, state.ZBattlefield)
	elves0 := moveOwnerCard(t, e, 0, elves, state.ZBattlefield)
	if bear0 == elves0 {
		t.Fatal("precondition: the two creatures are one object")
	}
	addMana(t, e, 0, "WWWWWW")
	castFirst(t, e, "cast")
	// Resolve until the placement ask poses.
	for i := 0; i < 20 && (e.Pending() == nil || e.Pending().Kind != decision.KArrange); i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		castFirst(t, e, "pass")
	}
	d := pendingArrange(t, e)
	if d.Player != 0 {
		t.Fatalf("arrange asked of seat %d, want the owner 0", d.Player)
	}
	offered := []state.ObjID{d.Options[0].Obj, d.Options[1].Obj}
	if !slices.Contains(offered, bear0) || !slices.Contains(offered, elves0) {
		t.Fatalf("offered = %v, want both seat-0 creatures %v %v", offered, bear0, elves0)
	}
	submitChoices(t, e, 1, 0) // reversed: not the offered order
	passUntilStackEmpty(t, e, 20)
	lib := e.G.Zone(state.ZLibrary, 0)
	got := lib[len(lib)-2:]
	if want := []state.ObjID{offered[1], offered[0]}; !slices.Equal(got, want) {
		t.Fatalf("library bottom = %v, want the answered order %v", got, want)
	}
	if slices.Equal(got, offered) {
		t.Fatal("precondition: the answer did not differ from the offered order")
	}
	replayCheck(t, e, cfg)
}

// TestTargetedChangeZoneTopArrangement: a targeted ChangeZone putting two of
// one owner's cards on top of their library poses the CR 401.4 arrange and
// places them reversed when answered so.
func TestTargetedChangeZoneTopArrangement(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spell := card(t, "Name:PicTopTwo\nManaCost:U\nTypes:Sorcery\n"+
		"A:SP$ ChangeZone | ValidTgts$ Creature | TargetMin$ 2 | TargetMax$ 2 | Origin$ Battlefield | Destination$ Library | LibraryPosition$ 0\nOracle:x\n")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	elves := mustCorpusCard(t, reg, "Llanowar Elves")
	e, cfg := censusEngine(t, 4713, []*cards.Card{spell, bear, elves}, []*cards.Card{bear})
	cardToHand(t, e, spell)
	bear0 := moveOwnerCard(t, e, 0, bear, state.ZBattlefield)
	elves0 := moveOwnerCard(t, e, 0, elves, state.ZBattlefield)
	addMana(t, e, 0, "U")
	castFirst(t, e, "cast")
	if d := e.Pending(); d == nil || d.Kind != "target" || d.Min != 2 || len(d.Options) != 2 {
		t.Fatalf("pending = %+v, want the two-target pick", d)
	}
	submitChoices(t, e, 0, 1)
	for i := 0; i < 20 && len(e.G.Stack) > 0 && e.Pending().Kind != decision.KArrange; i++ {
		castFirst(t, e, "pass")
	}
	d := pendingArrange(t, e)
	offered := []state.ObjID{d.Options[0].Obj, d.Options[1].Obj}
	if !slices.Contains(offered, bear0) || !slices.Contains(offered, elves0) {
		t.Fatalf("offered = %v, want both targets %v %v", offered, bear0, elves0)
	}
	submitChoices(t, e, 1, 0)
	passUntilStackEmpty(t, e, 20)
	got := e.G.Zone(state.ZLibrary, 0)[:2]
	if want := []state.ObjID{offered[1], offered[0]}; !slices.Equal(got, want) {
		t.Fatalf("library top = %v, want the answered order %v", got, want)
	}
	if slices.Equal(got, offered) {
		t.Fatal("precondition: the answer did not differ from the offered order")
	}
	replayCheck(t, e, cfg)
}
