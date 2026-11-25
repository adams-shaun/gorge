package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSupportActivatedTargetsAtActivation pins CR 701.41a's target timing
// for Support on an ACTIVATED ability (Joraga Auxiliary's "{4}{G}{W}:
// Support 2"): CR 602.2b makes the rest of activating an ability the
// casting process, so the "up to N other target creatures" choice is
// announced at activation (CR 601.2c through the cast target flow), never
// as a resolution counter_pick after priority has passed. The cast names
// two of the three other creatures; the third is never a target and the
// source itself is excluded by the "other".
func TestSupportActivatedTargetsAtActivation(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"joraga support activated","turn":1,"cr":["701.41a","602.2b"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Joraga Auxiliary","Temple Acolyte","Grizzly Bears"],` +
		`"library":["Forest","Forest"],"life":20},` +
		`"p1":{"battlefield":["Runeclaw Bear"],"life":20}},"steps":[` +
		`{"op":"activate","seat":0,"card":"p0:Joraga Auxiliary","mana":"CCCCGW","targets":["p0:Temple Acolyte","p1:Runeclaw Bear"],"ability_index":0},` +
		`{"op":"resolve"}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	// The activation itself must have asked for its targets (CR 601.2c with
	// 602.2b): a target decision on the activate step with both picks.
	// Without this the counters assertion below is not evidence of
	// activation-time targeting.
	asked := false
	for _, d := range res.Decisions {
		if d.Step == 0 && d.Kind == "target" && len(d.PickRefs) == 2 {
			asked = true
		}
	}
	if !asked {
		t.Fatalf("no two-pick target decision at the activation step: %+v", res.Decisions)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	counters := map[string]int32{}
	onBoard := map[string]bool{}
	for _, p := range last.Permanents {
		onBoard[p.Name] = true
		counters[p.Name] = p.Counters["P1P1"]
	}
	// Precondition: every creature is still on the battlefield, so a missing
	// counter is a real support-target check and not a zone change.
	for _, name := range []string{"Joraga Auxiliary", "Temple Acolyte", "Grizzly Bears", "Runeclaw Bear"} {
		if !onBoard[name] {
			t.Fatalf("precondition: %s left the battlefield: %v", name, onBoard)
		}
	}
	if got := counters["Temple Acolyte"]; got != 1 {
		t.Fatalf("Temple Acolyte +1/+1 counters = %d, want 1 (it was an activation-time support target)", got)
	}
	if got := counters["Runeclaw Bear"]; got != 1 {
		t.Fatalf("Runeclaw Bear +1/+1 counters = %d, want 1 (it was an activation-time support target)", got)
	}
	if got := counters["Grizzly Bears"]; got != 0 {
		t.Fatalf("Grizzly Bears +1/+1 counters = %d, want 0 (never a support target): %s", got, fmt.Sprint(counters))
	}
	if got := counters["Joraga Auxiliary"]; got != 0 {
		t.Fatalf("Joraga Auxiliary +1/+1 counters = %d, want 0 (the source is not one of the 'other' creatures): %s", got, fmt.Sprint(counters))
	}
}

// TestSupportTriggerPaidAnnouncesAtPush pins the same timing for the
// cost-window carrier: Jubilant Mascot's "you may pay {3}{W}. If you do,
// support 2" rides a trigger whose body is an ACTIVATED-ability SVar, so
// its targets are announced when the trigger goes on the stack (CR 603.3d)
// -- not as a resolution counter_pick after the pay window. Declining to
// pay leaves the announced targets unconsumed and nobody countered.
func TestSupportTriggerPaidAnnouncesAtPush(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"jubilant mascot support at push","turn":1,"cr":["701.41a","603.3d"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Jubilant Mascot","Temple Acolyte","Grizzly Bears"],` +
		`"library":["Forest","Forest"],"life":20},` +
		`"p1":{"battlefield":["Runeclaw Bear"],"life":20}},"steps":[` +
		`{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0","targets":["p0:Temple Acolyte","p1:Runeclaw Bear"]},` +
		`{"op":"mana","seat":0,"mana":"CCCW"},` +
		`{"op":"resolve","answers":[{"kind":"choose","pick":["pay"]}]},` +
		`{"op":"pass_to","seat":0,"step":"end","active":"p0"}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	// The trigger's push asked for the Support targets (CR 603.3d): a target
	// decision on the pass_to step (step 0) with both picks.
	asked := false
	for _, d := range res.Decisions {
		if d.Step == 0 && d.Kind == "target" && len(d.PickRefs) == 2 {
			asked = true
		}
	}
	if !asked {
		t.Fatalf("no two-pick target decision at the trigger push: %+v", res.Decisions)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	counters := map[string]int32{}
	onBoard := map[string]bool{}
	for _, p := range last.Permanents {
		onBoard[p.Name] = true
		counters[p.Name] = p.Counters["P1P1"]
	}
	for _, name := range []string{"Jubilant Mascot", "Temple Acolyte", "Grizzly Bears", "Runeclaw Bear"} {
		if !onBoard[name] {
			t.Fatalf("precondition: %s left the battlefield: %v", name, onBoard)
		}
	}
	if got := counters["Temple Acolyte"]; got != 1 {
		t.Fatalf("Temple Acolyte +1/+1 counters = %d, want 1 (announced at push, cost was paid)", got)
	}
	if got := counters["Runeclaw Bear"]; got != 1 {
		t.Fatalf("Runeclaw Bear +1/+1 counters = %d, want 1 (announced at push, cost was paid)", got)
	}
	if got := counters["Grizzly Bears"]; got != 0 {
		t.Fatalf("Grizzly Bears +1/+1 counters = %d, want 0 (never a support target): %s", got, fmt.Sprint(counters))
	}
	if got := counters["Jubilant Mascot"]; got != 0 {
		t.Fatalf("Jubilant Mascot +1/+1 counters = %d, want 0 (the source is not one of the 'other' creatures): %s", got, fmt.Sprint(counters))
	}
}
