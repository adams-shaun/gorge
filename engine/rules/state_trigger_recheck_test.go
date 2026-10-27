package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runStateTriggerScenario plays one inline audit scenario and returns the
// result, failing on a harness error, an expectation mismatch or a replay
// divergence -- the three ways these tests must be able to fail.
func runStateTriggerScenario(t *testing.T, sc string) OracleAuditResult {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil {
		t.Fatalf("scenario: err=%v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario fails: %v\ntranscript:\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	if res.ReplayDiff != "" {
		t.Fatalf("replay diff: %s", res.ReplayDiff)
	}
	return res
}

// TestStateTriggerNotRecheckedOnResolution is the CR 603.8 regression: a
// Mode$ Always state trigger that fires while the condition holds is not
// rechecked as it resolves. Synod Centurion's "when you control no other
// artifacts, sacrifice CARDNAME" fires when Cloudshift exiles the only other
// artifact (Bottle Gnomes), but Cloudshift returns Gnomes in the same
// resolution; the trigger is already on the stack and must still sacrifice
// the Centurion. Before the fix resolveTop's CR 603.4 gate re-evaluated the
// IsPresent$ clause, found Gnomes back, and exiled the ability with
// "fizzled: intervening-if no longer holds".
//
// XMage port (MIT): triggers/SynodCenturionTest.json#testWithFlicker.
func TestStateTriggerNotRecheckedOnResolution(t *testing.T) {
	t.Parallel()
	const sc = `{"name":"synod-flicker","cr":["603.8"],"why":"XMage SynodCenturionTest#testWithFlicker","setup":{"p0":{"hand":["Cloudshift","Synod Centurion"],"battlefield":["Plains","Plains","Plains","Plains","Plains","Plains","Bottle Gnomes"]}},"steps":[{"op":"resolve","seat":0},{"op":"cast","seat":0,"card":"p0:Synod Centurion","mana":"CCCC"},{"op":"pass_to","seat":0,"step":"main2","active":"p0"},{"op":"cast","seat":0,"card":"p0:Cloudshift","mana":"W","targets":["p0:Bottle Gnomes"]},{"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Bottle Gnomes"},"eq":1},{"count":{"seat":0,"zone":"graveyard","name":"Synod Centurion"},"eq":1}]}`
	runStateTriggerScenario(t, sc)
}

// TestStateTriggerStillFiresOnceAlone is the fire-time half: Synod Centurion
// entering with no other artifact matches its state condition, fires, and
// resolves its sacrifice. The trigger is not re-queued while the outstanding
// instance is on the stack (CR 603.8's "does not trigger again until it has
// left the stack"), so the Centurion is sacrificed exactly once. The
// battlefield/graveyard counts are the assertion; the runner would report a
// harness failure if the latch let the trigger enqueue without bound.
//
// XMage port (MIT): triggers/SynodCenturionTest.json#testAlone.
func TestStateTriggerStillFiresOnceAlone(t *testing.T) {
	t.Parallel()
	const sc = `{"name":"synod-alone","cr":["603.8"],"why":"XMage SynodCenturionTest#testAlone","setup":{"p0":{"hand":["Synod Centurion"],"battlefield":["Plains","Plains","Plains","Plains"]}},"steps":[{"op":"resolve","seat":0},{"op":"cast","seat":0,"card":"p0:Synod Centurion","mana":"CCCC"},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Synod Centurion"},"eq":0},{"count":{"seat":0,"zone":"graveyard","name":"Synod Centurion"},"eq":1}]}`
	runStateTriggerScenario(t, sc)
}

// TestInterveningIfStillRechecked is the control the Always exemption must
// not swallow: an ordinary CR 603.4 intervening-if is still rechecked as the
// ability resolves. Sunspire Gatekeepers entering with two Gates fires its
// "if you control two or more Gates" trigger; one Gate is destroyed while the
// trigger waits on the stack, so the IsPresent$ Gate.YouCtrl GE2 clause is
// false at resolution and the ability must fizzle without creating its Knight.
// The stack_size expectation proves the trigger really queued (the
// precondition the token assertion depends on). A globally disabled recheck
// would create the token and fail this test.
func TestInterveningIfStillRechecked(t *testing.T) {
	t.Parallel()
	const sc = `{"name":"gatekeepers-fizzle","cr":["603.4"],"why":"CR 603.4 intervening-if is rechecked at resolution","setup":{"p0":{"hand":["Sunspire Gatekeepers"],"battlefield":["Simic Guildgate","Selesnya Guildgate"]}},"steps":[{"op":"move","seat":0,"card":"p0:Sunspire Gatekeepers","to":"battlefield","expect":[{"stack_size":1}]},{"op":"move","seat":0,"card":"p0:Simic Guildgate","to":"graveyard","expect":[{"stack_size":1}]},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Knight Token"},"eq":0}]}`
	runStateTriggerScenario(t, sc)
}
