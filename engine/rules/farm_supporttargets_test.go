package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmSupportTargets pins CR 701.41a's target timing for Support on a
// spell (Shoulder to Shoulder: "Support 2. Draw a card."): Support N on an
// instant or sorcery puts a +1/+1 counter on each of up to N TARGET
// creatures, and those targets are announced as part of casting (CR 601.2c),
// never chosen as a resolution pick. The farm window SUP-s134-supp
// (xe:RAND:2:3:tape) shows gorge logging a choose_n counter_pick at
// resolution and countering Temple Acolyte and War Priest of Thune, while
// Forge leaves both alone because the support targets were never chosen at
// cast (Forge's CountersPutEffect Support branch installs TargetMin 0 /
// TargetMax N at cast).
//
// The cast names two of the three creatures; the third is never a target and
// must stay counterless. On origin/main the cast poses no target ask, so the
// scaffolded targets go unconsumed (and the resolution counters every
// creature instead).
func TestFarmSupportTargets(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm support targets","turn":1,"cr":["701.41a","601.2c"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Plains","Plains","Plains","Temple Acolyte","War Priest of Thune","Grizzly Bears"],` +
		`"hand":["Shoulder to Shoulder"],"library":["Forest","Forest"],"life":20},` +
		`"p1":{"battlefield":["Runeclaw Bear"],"life":20}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Shoulder to Shoulder","mana":"WWW","targets":["p0:Temple Acolyte","p1:Runeclaw Bear"]},` +
		`{"op":"resolve"}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	// The cast itself must have asked for its targets (CR 601.2c): a target
	// decision on step 0 with both scaffolded picks. Without this the
	// counters assertion below is not evidence of cast-time targeting.
	asked := false
	for _, d := range res.Decisions {
		if d.Step == 0 && d.Kind == "target" && len(d.PickRefs) == 2 {
			asked = true
		}
	}
	if !asked {
		t.Fatalf("no two-pick target decision at the cast step: %+v", res.Decisions)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	counters := map[string]int32{}
	for _, p := range last.Permanents {
		if len(p.Counters) > 0 {
			counters[p.Name] = p.Counters["P1P1"]
		}
	}
	// Precondition: all three creatures are still on the battlefield, so a
	// missing counter on the untargeted one is a real support target check.
	onBoard := map[string]bool{}
	for _, p := range last.Permanents {
		onBoard[p.Name] = true
	}
	for _, name := range []string{"Temple Acolyte", "War Priest of Thune", "Grizzly Bears"} {
		if !onBoard[name] {
			t.Fatalf("precondition: %s left the battlefield: %v", name, onBoard)
		}
	}
	if got := counters["Temple Acolyte"]; got != 1 {
		t.Fatalf("Temple Acolyte +1/+1 counters = %d, want 1 (it was a cast-time support target)", got)
	}
	if got := counters["Runeclaw Bear"]; got != 1 {
		t.Fatalf("Runeclaw Bear +1/+1 counters = %d, want 1 (it was a cast-time support target)", got)
	}
	if got := counters["Grizzly Bears"]; got != 0 {
		t.Fatalf("Grizzly Bears +1/+1 counters = %d, want 0 (never a support target): %s", got, fmt.Sprint(counters))
	}
}

// supportResponseScenario casts Shoulder to Shoulder at targets, lets p1
// Lightning Bolt the Runeclaw Bear in response, and resolves both.
func supportResponseScenario(t *testing.T, targets string) OracleResult {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm support targets recheck","turn":1,"cr":["701.41a","608.2b"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Plains","Plains","Plains","Temple Acolyte"],` +
		`"hand":["Shoulder to Shoulder"],"library":["Forest","Forest"],"life":20},` +
		`"p1":{"battlefield":["Runeclaw Bear"],"hand":["Lightning Bolt"],"mana":"R","life":20}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Shoulder to Shoulder","mana":"WWW","targets":[` + targets + `]},` +
		`{"op":"pass","seat":0},` +
		`{"op":"cast","seat":1,"card":"p1:Lightning Bolt","mana":"R","targets":["p1:Runeclaw Bear"]},` +
		`{"op":"resolve"},{"op":"resolve"}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	return res
}

func supportFinal(res OracleResult) (counters map[string]int32, onBoard map[string]bool, hand map[string]int) {
	last := res.Snapshots[len(res.Snapshots)-1]
	counters, onBoard, hand = map[string]int32{}, map[string]bool{}, map[string]int{}
	for _, p := range last.Permanents {
		onBoard[p.Name] = true
		counters[p.Name] = p.Counters["P1P1"]
	}
	for _, c := range last.Players[0].Hand {
		hand[c]++
	}
	return
}

// TestFarmSupportTargetsFizzle: Support's targets are real targets, so CR
// 608.2b applies at resolution. The sole target dies in response, so
// Shoulder to Shoulder does not resolve at all and its "Draw a card" never
// happens (Forge fizzles the SA it installed the targets on).
func TestFarmSupportTargetsFizzle(t *testing.T) {
	t.Parallel()
	res := supportResponseScenario(t, `"p1:Runeclaw Bear"`)
	counters, onBoard, hand := supportFinal(res)
	if onBoard["Runeclaw Bear"] {
		t.Fatalf("precondition: Runeclaw Bear survived the Bolt: %v", onBoard)
	}
	if counters["Temple Acolyte"] != 0 {
		t.Fatalf("Temple Acolyte got %d counters, never a target", counters["Temple Acolyte"])
	}
	if n := countHand(hand); n != 0 {
		t.Fatalf("p0 drew a card from a spell whose only target was illegal (CR 608.2b): hand %v", hand)
	}
}

// TestFarmSupportTargetsPartialFizzle: of two Support targets one dies in
// response; the spell resolves (CR 608.2b: one target is still legal), the
// survivor gets its counter, and the draw happens.
func TestFarmSupportTargetsPartialFizzle(t *testing.T) {
	t.Parallel()
	res := supportResponseScenario(t, `"p0:Temple Acolyte","p1:Runeclaw Bear"`)
	counters, onBoard, hand := supportFinal(res)
	if onBoard["Runeclaw Bear"] || !onBoard["Temple Acolyte"] {
		t.Fatalf("precondition: want Bear dead, Acolyte alive: %v", onBoard)
	}
	if counters["Temple Acolyte"] != 1 {
		t.Fatalf("Temple Acolyte +1/+1 counters = %d, want 1 (surviving target)", counters["Temple Acolyte"])
	}
	if n := countHand(hand); n != 1 {
		t.Fatalf("p0 should have drawn once: hand %v", hand)
	}
}

func countHand(hand map[string]int) int {
	n := 0
	for _, c := range hand {
		n += c
	}
	return n
}
