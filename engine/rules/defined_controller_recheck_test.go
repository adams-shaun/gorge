package rules

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// definedControllerOffer asks sa's target from source and returns the offered
// object ids, sorted; nil when no decision was raised.
func definedControllerOffer(e *Engine, source state.ObjID, sa *cards.SA) []state.ObjID {
	e.pending = nil
	e.askTarget(0, source, sa)
	d := e.Pending()
	if d == nil {
		return nil
	}
	var ids []state.ObjID
	for _, o := range d.Options {
		ids = append(ids, o.Obj)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func definedControllerObjs(ts []state.Target) []state.ObjID {
	var ids []state.ObjID
	for _, t := range ts {
		ids = append(ids, t.Obj)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func TestDefinedControllerRecheckFearOfBurningAlive(t *testing.T) {
	reg := freshCorpusRegistry(t, "f/fear_of_burning_alive.txt")
	card, ok := reg.Lookup("Fear of Burning Alive")
	if !ok || len(card.Faces) == 0 {
		t.Fatal("precondition: Fear of Burning Alive is missing from the corpus")
	}
	sa := cards.ResolveSVar(card.Faces[0].SVars, "TrigDmg")
	if sa == nil || sa.API != "DealDamage" || sa.Params["TargetsWithDefinedController"] != "TriggeredTarget" {
		t.Fatalf("precondition: Fear target SVar = %+v; want DealDamage with TriggeredTarget", sa)
	}

	e := newSeats(t, 2)
	fear := e.G.AddObject(card, 0).ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: fear, From: state.ZLibrary, To: state.ZBattlefield})
	mine := putBattlefield(t, e, 0, "Name:Mine\nTypes:Creature\nPT:1/1\nOracle:x\n")
	theirs := putBattlefield(t, e, 1, "Name:Theirs\nTypes:Creature\nPT:1/1\nOracle:x\n")
	if e.G.Obj(mine).Controller != 0 || e.G.Obj(theirs).Controller != 1 {
		t.Fatal("precondition: Mine must be seat 0's and Theirs seat 1's")
	}
	if !e.matchesSpec(sa.Params["ValidTgts"], mine, effects.SpecContext{}) || !e.matchesSpec(sa.Params["ValidTgts"], theirs, effects.SpecContext{}) {
		t.Fatal("precondition: both creatures must match ValidTgts")
	}

	// The damaged opponent ("that player") is seat 1.
	e.triggerContexts = map[state.ObjID]effects.TriggerContext{
		fear: {TriggerTarget: state.Target{Player: 1, IsPlayer: true}},
	}
	if got := definedControllerOffer(e, fear, sa); !sameObjIDs(got, []state.ObjID{theirs}) {
		t.Fatalf("offer = %v, want only Theirs %d", got, theirs)
	}
	zones := targetZones(sa)
	initial := e.legalTargets([]state.Target{{Obj: mine}, {Obj: theirs}}, sa, zones, 0, fear, fear)
	if got := definedControllerObjs(initial); !sameObjIDs(got, []state.ObjID{theirs}) {
		t.Fatalf("recheck = %v, want only Theirs %d", got, theirs)
	}

	// Theirs changes controller to seat 0 before the trigger resolves.
	e.emit(events.Event{Kind: events.ControlChange, Obj: theirs, Player: 0})
	if e.G.Obj(theirs).Controller != 0 {
		t.Fatal("precondition: ControlChange did not give Theirs to seat 0")
	}
	if changed := e.legalTargets([]state.Target{{Obj: theirs}}, sa, zones, 0, fear, fear); len(changed) != 0 {
		t.Fatalf("recheck after control change = %+v, want no legal targets", changed)
	}
}

func TestDefinedControllerRecheckDriverOfTheDead(t *testing.T) {
	reg := freshCorpusRegistry(t, "d/driver_of_the_dead.txt")
	card, ok := reg.Lookup("Driver of the Dead")
	if !ok || len(card.Faces) == 0 {
		t.Fatal("precondition: Driver of the Dead is missing from the corpus")
	}
	sa := cards.ResolveSVar(card.Faces[0].SVars, "TrigChange")
	if sa == nil || sa.API != "ChangeZone" || sa.Params["TargetsWithDefinedController"] != "TriggeredCardController" {
		t.Fatalf("precondition: Driver target SVar = %+v; want ChangeZone with TriggeredCardController", sa)
	}

	e := newSeats(t, 2)
	driver := e.G.AddObject(card, 0).ID
	mine := putGraveyard(t, e, 0, "Name:Mine\nTypes:Creature\nManaCost:1\nPT:1/1\nOracle:x\n")
	theirs := putGraveyard(t, e, 1, "Name:Theirs\nTypes:Creature\nManaCost:1\nPT:1/1\nOracle:x\n")
	if e.G.Obj(mine).Zone != state.ZGraveyard || e.G.Obj(theirs).Zone != state.ZGraveyard ||
		e.G.Obj(mine).Controller != 0 || e.G.Obj(theirs).Controller != 1 {
		t.Fatal("precondition: Mine in seat 0's graveyard, Theirs in seat 1's")
	}
	if !e.matchesSpec(sa.Params["ValidTgts"], mine, effects.SpecContext{}) || !e.matchesSpec(sa.Params["ValidTgts"], theirs, effects.SpecContext{}) {
		t.Fatal("precondition: both cards must match ValidTgts")
	}

	e.triggerContexts = map[state.ObjID]effects.TriggerContext{
		driver: {TriggerCard: driver, TriggerCardController: state.Target{Player: 0, IsPlayer: true}},
	}
	if p, ok := effects.TriggeredCardController(e.G, e.triggerContexts[driver], nil); !ok || p != 0 {
		t.Fatalf("precondition: TriggeredCardController = %d (ok=%v), want seat 0", p, ok)
	}
	if got := definedControllerOffer(e, driver, sa); !sameObjIDs(got, []state.ObjID{mine}) {
		t.Fatalf("offer = %v, want only Mine %d", got, mine)
	}
	got := e.legalTargets([]state.Target{{Obj: mine}, {Obj: theirs}}, sa, targetZones(sa), 0, driver, driver)
	if ids := definedControllerObjs(got); !sameObjIDs(ids, []state.ObjID{mine}) {
		t.Fatalf("recheck = %v, want only Mine %d", ids, mine)
	}
}

func TestDefinedControllerRecheckUnresolvedReferentMatchesOffer(t *testing.T) {
	reg := freshCorpusRegistry(t, "f/fear_of_burning_alive.txt")
	card, ok := reg.Lookup("Fear of Burning Alive")
	if !ok || len(card.Faces) == 0 {
		t.Fatal("precondition: Fear of Burning Alive is missing from the corpus")
	}
	sa := cards.ResolveSVar(card.Faces[0].SVars, "TrigDmg")
	if sa == nil || sa.Params["TargetsWithDefinedController"] != "TriggeredTarget" {
		t.Fatalf("precondition: Fear target SVar = %+v", sa)
	}
	e := newSeats(t, 2)
	fear := e.G.AddObject(card, 0).ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: fear, From: state.ZLibrary, To: state.ZBattlefield})
	mine := putBattlefield(t, e, 0, "Name:Mine\nTypes:Creature\nPT:1/1\nOracle:x\n")
	theirs := putBattlefield(t, e, 1, "Name:Theirs\nTypes:Creature\nPT:1/1\nOracle:x\n")

	// No trigger role bound: the referent is unresolved.
	e.triggerContexts = make(map[state.ObjID]effects.TriggerContext)
	sc := e.targetSpecContext(fear, fear, 0)
	_, resolved, failClosed, nonTriggered := e.definedControllerPlayer(effects.TargetsOf(sa).DefinedController, sc)
	e.releaseSpecEnv()
	if resolved || failClosed || nonTriggered {
		t.Fatalf("precondition: referent resolved=%v failClosed=%v nonTriggered=%v, want an unresolved admit-all arm", resolved, failClosed, nonTriggered)
	}
	offer := definedControllerOffer(e, fear, sa)
	if !sameObjIDs(offer, []state.ObjID{mine, theirs}) && !sameObjIDs(offer, []state.ObjID{theirs, mine}) {
		t.Fatalf("offer = %v, want both creatures (admit-all arm)", offer)
	}
	recheck := definedControllerObjs(e.legalTargets([]state.Target{{Obj: mine}, {Obj: theirs}}, sa, targetZones(sa), 0, fear, fear))
	if !sameObjIDs(recheck, offer) {
		t.Fatalf("recheck = %v, offer = %v; want identical", recheck, offer)
	}
}
