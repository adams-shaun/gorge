package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// drainAccepting passes priority until the stack is empty, accepting every
// optional-trigger election and answering any other choice with the option
// naming pick (or, failing that, the first option). It returns the number of
// optional elections it accepted.
func drainAccepting(t *testing.T, e *Engine, pick state.ObjID, limit int) int {
	t.Helper()
	accepted := 0
	for n := 0; n < limit && !e.G.Over; n++ {
		d := e.Pending()
		if d == nil {
			if len(e.G.Stack) != 0 {
				t.Fatalf("no decision with a live stack: %+v", e.G.Stack)
			}
			return accepted
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				return accepted
			}
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
			continue
		}
		if d.Kind == decision.KTriggerOptional {
			accepted++
			submitChoices(t, e, d.Options[0].Index) // yes
			continue
		}
		if len(d.Options) == 0 {
			submitChoices(t, e)
			continue
		}
		idx := d.Options[0].Index
		for _, o := range d.Options {
			if pick != 0 && o.Obj == pick {
				idx = o.Index
			}
		}
		submitChoices(t, e, idx)
	}
	t.Fatalf("drain did not settle in %d steps", limit)
	return accepted
}

func inSeatZone(e *Engine, z state.Zone, p state.PlayerID, id state.ObjID) bool {
	for _, x := range e.G.Zone(z, p) {
		if x == id {
			return true
		}
	}
	return false
}

// renownAttack attacks with id at seat 0's declare-attackers step and
// resolves combat damage plus the Renown trigger.
func renownAttack(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.askAttackers()
	submitAttackers(t, e, id)
	drainCombatDamagePriority(t, e)
}

func TestRelicSeekerBecomeRenownedSearchesForEquipment(t *testing.T) {
	t.Parallel()
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Relic Seeker", "Bonesplitter"}, nil, nil, nil)
	equip := findBattlefield(t, e, 0, "Bonesplitter", 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: equip, From: state.ZBattlefield, To: state.ZLibrary})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	id := findBattlefield(t, e, 0, "Relic Seeker", 0)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Renowned {
		t.Fatalf("precondition: Relic Seeker unrenowned on the battlefield: %+v", o)
	}
	if !inSeatZone(e, state.ZLibrary, 0, equip) {
		t.Fatal("precondition: Bonesplitter is not in seat 0's library")
	}
	for _, h := range e.G.Zone(state.ZHand, 0) {
		if slices.Contains(e.G.Obj(h).Face().Types, "Equipment") {
			t.Fatalf("precondition: hand already holds an Equipment (%s)", e.G.Obj(h).Face().Name)
		}
	}
	renownAttack(t, e, id)
	accepted := drainAccepting(t, e, equip, 60)
	if !e.G.Obj(id).Renowned {
		t.Fatal("Relic Seeker did not become renowned")
	}
	if marks := renownMarks(e, id); len(marks) != 1 {
		t.Fatalf("want exactly one Renowned mark, got %+v", marks)
	}
	if accepted != 1 {
		t.Fatalf("accepted %d optional elections, want 1 (the BecomeRenowned trigger)", accepted)
	}
	if !inSeatZone(e, state.ZHand, 0, equip) {
		t.Fatalf("Bonesplitter is in zone %v, want seat 0's hand", e.G.Obj(equip).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestValeronWardensBecomeRenownedDrawsForAnotherCreature(t *testing.T) {
	t.Parallel()
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Valeron Wardens", "Knight of the Pilgrim's Road"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	wardens := findBattlefield(t, e, 0, "Valeron Wardens", 0)
	knight := findBattlefield(t, e, 0, "Knight of the Pilgrim's Road", 0)
	if e.G.Obj(wardens).Renowned || e.G.Obj(knight).Renowned {
		t.Fatal("precondition: a creature is already renowned")
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	renownAttack(t, e, knight)
	drainAccepting(t, e, 0, 60)
	if !e.G.Obj(knight).Renowned || e.G.Obj(wardens).Renowned {
		t.Fatalf("want only the Knight renowned: knight=%v wardens=%v", e.G.Obj(knight).Renowned, e.G.Obj(wardens).Renowned)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 1 {
		t.Fatalf("seat 0 drew %d cards, want exactly 1", got)
	}
	replayCheck(t, e, cfg)
}

// dungeonBoard deals seat 0 a deck holding the corpus card name, moves it to
// zone, puts a Lost Mine of Phandelver into the completing seat's command
// zone and parks the clock at seat 0's first main phase.
func dungeonBoard(t *testing.T, name string, zone state.Zone, completer state.PlayerID) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck0 := append([]*cards.Card{mustCorpusCard(t, reg, name)}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 23, Names: []string{"a", "b"},
		Tokens: reg.Tokens,
		Decks:  [][]*cards.Card{deck0, mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := crAbortMove(t, e, 0, name, zone)
	if e.G.Obj(id).Zone != zone {
		t.Fatalf("precondition: %s in zone %v, want %v", name, e.G.Obj(id).Zone, zone)
	}
	if d := e.PutDungeon(completer, "lost_mine_of_phandelver"); d == 0 || e.G.Obj(d).Zone != state.ZCommand {
		t.Fatalf("precondition: no active dungeon for seat %d", completer)
	}
	return e, cfg, id
}

func wolves(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if f := o.Face(); o.IsToken && f != nil && slices.Contains(f.Types, "Wolf") && f.PT == "2/2" && f.Colors == "green" {
			n++
		}
	}
	return n
}

func TestVarisDungeonCompletedCreatesWolf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		completer state.PlayerID
		want      int
	}{{"you", 0, 1}, {"opponent", 1, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg, _ := dungeonBoard(t, "Varis, Silverymoon Ranger", state.ZBattlefield, tc.completer)
			if n := wolves(e, 0); n != 0 {
				t.Fatalf("precondition: %d Wolves before the completion", n)
			}
			e.CompleteDungeon(tc.completer)
			if e.G.Players[tc.completer].CompletedDungeons != 1 {
				t.Fatalf("precondition: seat %d did not complete the dungeon", tc.completer)
			}
			e.priorityRound()
			drainAccepting(t, e, 0, 40)
			if got := wolves(e, 0); got != tc.want {
				t.Fatalf("seat %d completing: %d Wolves on seat 0's battlefield, want %d", tc.completer, got, tc.want)
			}
			replayCheck(t, e, cfg)
		})
	}
}

func TestDungeonCrawlerReturnsFromGraveyardOnCompletion(t *testing.T) {
	t.Parallel()
	e, cfg, id := dungeonBoard(t, "Dungeon Crawler", state.ZGraveyard, 0)
	e.CompleteDungeon(0)
	e.priorityRound()
	if accepted := drainAccepting(t, e, 0, 40); accepted != 1 {
		t.Fatalf("accepted %d optional elections, want 1", accepted)
	}
	if !inSeatZone(e, state.ZHand, 0, id) {
		t.Fatalf("Dungeon Crawler is in zone %v, want seat 0's hand", e.G.Obj(id).Zone)
	}
	replayCheck(t, e, cfg)
}
