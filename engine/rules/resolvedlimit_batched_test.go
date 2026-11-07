package rules

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const resolvedLimitBatchedHead = "Name:Test Limited\nManaCost:1\nTypes:Artifact\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature | Execute$ TrigGain | TriggerZones$ Battlefield | ResolvedLimit$ 1"
const resolvedLimitBatchedTail = "\nSVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n"

// batchedLimitRun queues two triggers of one ResolvedLimit$ 1 line from one
// event batch, resolves both, and returns the life gained plus the number of
// optional asks posed.
func batchedLimitRun(t *testing.T, optional bool) (gained, asks int) {
	t.Helper()
	extra := ""
	if optional {
		extra = " | OptionalDecider$ You"
	}
	e := combatEngine(t)
	src := onBoardCard(t, e, 0, card(t, resolvedLimitBatchedHead+extra+resolvedLimitBatchedTail))
	before := e.G.Players[0].Life
	enterCreature(t, e, 0, card(t, resolvedLimitVillain))
	enterCreature(t, e, 0, card(t, resolvedLimitVillain))
	if len(e.pendingTriggers) != 2 {
		t.Fatalf("want 2 batched triggers queued, got %d", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	if n := pushCount(e, src); n != 2 {
		t.Fatalf("want both triggers on the stack, got %d", n)
	}
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			e.resolveTop()
			continue
		}
		if d.Kind == decision.KTriggerOptional {
			asks++
		}
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if d.Kind == decision.KPriority && o.Kind == "pass" {
				pick = o.Index
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
			t.Fatal(err)
		}
	}
	return int(e.G.Players[0].Life - before), asks
}

func TestResolvedLimitBatchedTriggers(t *testing.T) {
	t.Parallel()
	gained, asks := batchedLimitRun(t, true)
	if gained != 1 || asks != 1 {
		t.Fatalf("optional: gained %d life with %d asks, want 1 and 1", gained, asks)
	}
}

func TestResolvedLimitBatchedMandatoryTriggers(t *testing.T) {
	t.Parallel()
	gained, asks := batchedLimitRun(t, false)
	if gained != 1 || asks != 0 {
		t.Fatalf("mandatory: gained %d life with %d asks, want 1 and 0", gained, asks)
	}
}

func TestResolvedLimitFarmWindowsEOE196(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/EOE-s196_w14.scenario.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), raw)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res.Snapshots[len(res.Snapshots)-1])
	var snap struct {
		Players []struct {
			Seat         int   `json:"seat"`
			Hand         []any `json:"hand"`
			LibraryCount int   `json:"library_count"`
		} `json:"players"`
	}
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Players) == 0 || snap.Players[0].Seat != 0 {
		t.Fatalf("no seat 0 in the last snapshot: %s", b)
	}
	if p := snap.Players[0]; len(p.Hand) != 3 || p.LibraryCount != 25 {
		t.Fatalf("seat 0 hand %d library %d, want 3 and 25", len(p.Hand), p.LibraryCount)
	}
}
