package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// presentPlayerKeys is the suffixed parameter set IsPresent$ reaches
// presentClauseHolds through (and IsPresent2$ its own in
// triggerConditionHoldsWithSVars).
var presentPlayerKeys = []string{"IsPresent", "PresentCompare", "PresentDefined", "PresentZone", "PresentPlayer"}

const presentPlayerDesertSrc = "Name:Triage Desert\nTypes:Land Desert\nOracle:x\n"

func presentPlayerClause(e *Engine, tr cards.Trigger, source state.ObjID, you state.PlayerID) bool {
	return e.presentClauseHolds(tr, source, you, nil,
		presentPlayerKeys[0], presentPlayerKeys[1], presentPlayerKeys[2], presentPlayerKeys[3], presentPlayerKeys[4])
}

// presentPlayerGixianProbe is the brief's probe board: Gixian Skullflayer on
// seat 0's battlefield and n Grizzly Bears on bearsSeat's battlefield, moved
// to bearsSeat's graveyard through logged MoveZone events.
func presentPlayerGixianProbe(t *testing.T, bearsSeat state.PlayerID, n int) (*Engine, state.ObjID, *cards.Trigger) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	gix := onBoardCard(t, e, 0, lookup(t, reg, "Gixian Skullflayer"))
	if o := e.G.Obj(gix); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Gixian Skullflayer %d is not on seat 0's battlefield", gix)
	}
	bears := lookup(t, reg, "Grizzly Bears")
	for i := 0; i < n; i++ {
		id := onBoardCard(t, e, bearsSeat, bears)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Grizzly Bears %d is not on seat %d's battlefield", id, bearsSeat)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	}
	var tr *cards.Trigger
	face := e.G.Obj(gix).Face()
	for i := range face.Triggers {
		candidate := &face.Triggers[i]
		if strings.TrimSpace(candidate.Params["IsPresent"]) != "" &&
			strings.TrimSpace(candidate.Params["PresentPlayer"]) == "You" {
			tr = candidate
			break
		}
	}
	if tr == nil {
		t.Fatal("precondition: Gixian Skullflayer has no IsPresent$ trigger carrying PresentPlayer$ You")
	}
	return e, gix, tr
}

// TestPresentPlayerGixianCountsOnlyYourGraveyard is the brief's measured
// probe: with three creature cards only in seat 1's graveyard the clause must
// not hold, and with three in seat 0's it must.
func TestPresentPlayerGixianCountsOnlyYourGraveyard(t *testing.T) {
	t.Parallel()
	e, gix, tr := presentPlayerGixianProbe(t, 1, 3)
	if own := len(e.G.Zone(state.ZGraveyard, 0)); own != 0 {
		t.Fatalf("precondition: seat 0's graveyard has %d cards, want 0", own)
	}
	if opp := len(e.G.Zone(state.ZGraveyard, 1)); opp != 3 {
		t.Fatalf("precondition: seat 1's graveyard has %d cards, want 3", opp)
	}
	if presentPlayerClause(e, *tr, gix, 0) {
		t.Fatal("the present clause held with three creature cards only in seat 1's graveyard; present player You must count only seat 0's")
	}

	e2, gix2, tr2 := presentPlayerGixianProbe(t, 0, 3)
	if own := len(e2.G.Zone(state.ZGraveyard, 0)); own != 3 {
		t.Fatalf("precondition: seat 0's graveyard has %d cards, want 3", own)
	}
	if opp := len(e2.G.Zone(state.ZGraveyard, 1)); opp != 0 {
		t.Fatalf("precondition: seat 1's graveyard has %d cards, want 0", opp)
	}
	if !presentPlayerClause(e2, *tr2, gix2, 0) {
		t.Fatal("the present clause rejected three creature cards in seat 0's own graveyard")
	}
}

// presentPlayerGixianUpkeepGame seeds a real Gixian Skullflayer and three
// Grizzly Bears and moves them through logged events, so replayCheck can
// rebuild the game from the log alone afterwards.
func presentPlayerGixianUpkeepGame(t *testing.T, seed uint64, bearsSeat state.PlayerID) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	gix := lookup(t, reg, "Gixian Skullflayer")
	bears := lookup(t, reg, "Grizzly Bears")
	seat0 := []*cards.Card{gix}
	seat1 := []*cards.Card{}
	target := &seat0
	if bearsSeat == 1 {
		target = &seat1
	}
	for i := 0; i < 3; i++ {
		*target = append(*target, bears)
	}
	e, cfg := tokenReplGameSeats(t, seed, seat0, seat1)
	gixID := moveByName(t, e, 0, "Gixian Skullflayer", state.ZBattlefield)
	if o := e.G.Obj(gixID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Gixian Skullflayer %d is not on seat 0's battlefield", gixID)
	}
	for i := 0; i < 3; i++ {
		id := moveByName(t, e, bearsSeat, "Grizzly Bears", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Grizzly Bears %d is not on seat %d's battlefield", id, bearsSeat)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	}
	return e, cfg, gixID
}

// TestPresentPlayerGixianUpkeepTriggerFlow drives to seat 0's upkeep and
// checks the real trigger end to end: no counter from an opponent-only
// graveyard, one counter from the controller's own.
func TestPresentPlayerGixianUpkeepTriggerFlow(t *testing.T) {
	t.Parallel()
	t.Run("opponent graveyard only", func(t *testing.T) {
		e, cfg, gix := presentPlayerGixianUpkeepGame(t, 185, 1)
		if opp := len(e.G.Zone(state.ZGraveyard, 1)); opp != 3 {
			t.Fatalf("precondition: seat 1's graveyard has %d cards, want 3", opp)
		}
		if own := len(e.G.Zone(state.ZGraveyard, 0)); own != 0 {
			t.Fatalf("precondition: seat 0's graveyard has %d cards, want 0", own)
		}
		driveToStep(t, e, 3, 0, state.StepUpkeep)
		passUntilStackEmpty(t, e, 40)
		if got := e.G.Obj(gix).Counter("P1P1"); got != 0 {
			t.Fatalf("Gixian Skullflayer got %d +1/+1 counters from an opponent-only graveyard, want 0", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("controller graveyard", func(t *testing.T) {
		e, cfg, gix := presentPlayerGixianUpkeepGame(t, 186, 0)
		if own := len(e.G.Zone(state.ZGraveyard, 0)); own != 3 {
			t.Fatalf("precondition: seat 0's graveyard has %d cards, want 3", own)
		}
		driveToStep(t, e, 3, 0, state.StepUpkeep)
		passUntilStackEmpty(t, e, 40)
		if got := e.G.Obj(gix).Counter("P1P1"); got != 1 {
			t.Fatalf("Gixian Skullflayer got %d +1/+1 counters from its controller's own graveyard, want 1", got)
		}
		replayCheck(t, e, cfg)
	})
}

// TestPresentPlayerDesertOpponentGraveyardDoesNotCount pins the
// PresentZone$ Battlefield,Graveyard carrier: a Desert in an opponent's
// graveyard or under an opponent's control must not satisfy the clause, while
// one in the controller's own graveyard must.
func TestPresentPlayerDesertOpponentGraveyardDoesNotCount(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cases := []struct {
		name  string
		seat  state.PlayerID
		zone  state.Zone
		holds bool
	}{
		{"opponent graveyard", 1, state.ZGraveyard, false},
		{"opponent battlefield", 1, state.ZBattlefield, false},
		{"controller graveyard", 0, state.ZGraveyard, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := layerEngine(t)
			hold := onBoardCard(t, e, 0, lookup(t, reg, "Deserts Hold"))
			if o := e.G.Obj(hold); o == nil || o.Zone != state.ZBattlefield {
				t.Fatalf("precondition: Desert's Hold %d is not on the battlefield", hold)
			}
			var tr *cards.Trigger
			face := e.G.Obj(hold).Face()
			for i := range face.Triggers {
				candidate := &face.Triggers[i]
				if strings.TrimSpace(candidate.Params["PresentZone"]) == "Battlefield,Graveyard" &&
					strings.TrimSpace(candidate.Params["PresentPlayer"]) == "You" {
					tr = candidate
					break
				}
			}
			if tr == nil {
				t.Fatal("precondition: Desert's Hold has no Battlefield,Graveyard IsPresent$ trigger carrying PresentPlayer$ You")
			}
			if presentPlayerClause(e, *tr, hold, 0) {
				t.Fatal("precondition: the clause held with no Desert anywhere")
			}
			switch tc.zone {
			case state.ZGraveyard:
				inZoneCard(t, e, tc.seat, state.ZGraveyard, presentPlayerDesertSrc)
			default:
				onBoardCard(t, e, tc.seat, card(t, presentPlayerDesertSrc))
			}
			if presentPlayerClause(e, *tr, hold, 0) != tc.holds {
				t.Fatalf("with a Desert in seat %d's %v the clause holds=%v, want %v",
					tc.seat, tc.zone, !tc.holds, tc.holds)
			}
		})
	}
}

// TestPresentPlayerAbsentStillCountsEveryone keeps the Any/absent default:
// a clause with no PresentPlayer$ continues to count every living seat's
// graveyard.
func TestPresentPlayerAbsentStillCountsEveryone(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	source := onBoard(t, e, 0, "Name:Present Watcher\nTypes:Enchantment\nOracle:x\n")
	inZoneCard(t, e, 1, state.ZGraveyard, "Name:Opponent Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	tr := cards.Trigger{Params: map[string]string{
		"IsPresent":   "Creature",
		"PresentZone": "Graveyard",
	}}
	if _, ok := tr.Params["PresentPlayer"]; ok {
		t.Fatal("precondition: the fixture trigger carries PresentPlayer$")
	}
	if own := len(e.G.Zone(state.ZGraveyard, 0)); own != 0 {
		t.Fatalf("precondition: seat 0's graveyard has %d cards, want 0", own)
	}
	if opp := len(e.G.Zone(state.ZGraveyard, 1)); opp != 1 {
		t.Fatalf("precondition: seat 1's graveyard has %d cards, want 1", opp)
	}
	if !presentPlayerClause(e, tr, source, 0) {
		t.Fatal("a present clause with no PresentPlayer$ stopped counting seat 1's graveyard; absent must stay Any")
	}
}
