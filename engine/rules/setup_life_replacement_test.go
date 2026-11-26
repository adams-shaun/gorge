package rules

// A setup life total is a stated outside fact, not a game event: no CR 614
// replacement (Angel of Vitality's "plus 1") may rewrite it, and no "whenever
// you gain life" trigger may fire for it.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const setupLifeAngelScript = "Name:Setup Angel\nManaCost:2 W\nTypes:Creature Angel\nPT:2/2\n" +
	"R:Event$ GainLife | ActiveZones$ Battlefield | ValidPlayer$ You | ReplaceWith$ SetupAngelGain | Description$ x\n" +
	"SVar:SetupAngelGain:DB$ ReplaceEffect | VarName$ LifeGained | VarValue$ SetupAngelX\n" +
	"SVar:SetupAngelX:ReplaceCount$LifeGained/Plus.1\n" +
	"T:Mode$ LifeGained | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ SetupAngelDraw | TriggerDescription$ x\n" +
	"SVar:SetupAngelDraw:DB$ Draw | NumCards$ 1 | Defined$ You\nOracle:x\n"

const setupLifeAngelScenario = `{
  "name": "setup-life-angel",
  "setup": {"p0": {"battlefield": ["Angel of Vitality"], "life": 26}},
  "steps": [],
  "expect": [{"life": {"p0": 26}}]
}`

// Ajani's Pridemate is a corpus card whose "whenever you gain life" trigger
// puts a +1/+1 counter on it: a setup LifeChange that still queued triggers
// would resolve it during setup and end with the counter present.
const setupLifePridemateScenario = `{
  "name": "setup-life-pridemate",
  "setup": {"p0": {"battlefield": ["Ajani's Pridemate"], "life": 26}},
  "steps": [],
  "expect": [{"life": {"p0": 26}}]
}`

func TestOracleSetupLifeQueuesNoGainTrigger(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(setupLifePridemateScenario))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	var pridemate state.ObjID
	for _, id := range run.e.G.Zone(state.ZBattlefield, state.PlayerID(0)) {
		if run.e.G.Obj(id).Face().Name == "Ajani's Pridemate" {
			pridemate = id
		}
	}
	if pridemate == 0 {
		t.Fatal("precondition: Ajani's Pridemate is not on p0's battlefield")
	}
	if n := run.e.G.Obj(pridemate).Counter("P1P1"); n != 0 {
		t.Fatalf("setup life queued the Pridemate trigger: %d P1P1 counters after setup", n)
	}
}

func TestOracleSetupLifeIgnoresGainReplacement(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(setupLifeAngelScenario))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	if got := run.e.G.Players[0].Life; got != 26 {
		t.Fatalf("p0 life = %d, want 26 (setup life is not a gain)", got)
	}
}

func TestStartLifeIgnoresGainReplacement(t *testing.T) {
	life := int32(26)
	start := &StartSpec{Seats: []StartSeat{{Battlefield: []int{2}, Life: &life}}}
	e := New(startConfig(t, 5, start, [2]map[int]string{{2: setupLifeAngelScript}, {}}))
	if got := e.G.Obj(3).Face().Name; got != "Setup Angel" {
		t.Fatalf("precondition: object 3 is %q, not the angel", got)
	}
	if got := e.G.Players[0].Life; got != 26 {
		t.Fatalf("p0 life = %d, want 26 (setup life is not a gain)", got)
	}
	// The angel's LifeGained draw trigger must not fire either: the start
	// settle drops the triggers the setup queued, so none may be left pending
	// (a left-over pending draw would resolve on the first Advance) and the
	// hand never grows. Compare against the same seed with no life override
	// (no LifeChange at all): the hands must match.
	if len(e.pendingTriggers) != 0 || e.orderedTriggers != 0 {
		t.Fatalf("setup life left %d trigger(s) pending (ordered=%d): the settle must drop them",
			len(e.pendingTriggers), e.orderedTriggers)
	}
	plain := New(startConfig(t, 5, &StartSpec{Seats: []StartSeat{{Battlefield: []int{2}}}},
		[2]map[int]string{{2: setupLifeAngelScript}, {}}))
	if got, want := len(e.G.Zone(state.ZHand, 0)), len(plain.G.Zone(state.ZHand, 0)); got != want {
		t.Fatalf("p0 hand = %d cards, want %d (the LifeGained draw trigger fired)", got, want)
	}
}
