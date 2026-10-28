package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func tappedAfterUntapRun(t *testing.T, scenario string) (*oracleRun, []string, []string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(reg, sc)
	return run, fails, transcript
}

func TestOracleSetupTappedAfterUntapSurvivesUntapStep(t *testing.T) {
	for _, turn := range []string{"1", "3"} {
		run, fails, transcript := tappedAfterUntapRun(t, `{"name":"taut","turn":`+turn+`,"setup":{"p0":{"battlefield":["Grizzly Bears","Grizzly Bears","Forest"],"tapped_after_untap":["Grizzly Bears#2","Forest"]},"p1":{"battlefield":["Grizzly Bears"],"tapped_after_untap":["Grizzly Bears"]}},"steps":[]}`)
		if len(fails) != 0 {
			t.Fatalf("turn %s: scenario failed: %v\n%s", turn, fails, strings.Join(transcript, "\n"))
		}
		want := map[string]bool{"p0:Grizzly Bears": false, "p0:Grizzly Bears#2": true, "p0:Forest": true, "p1:Grizzly Bears": true}
		for ref, tapped := range want {
			id, ok := run.refs[ref]
			if !ok {
				t.Fatalf("turn %s: ref %s unbound", turn, ref)
			}
			if got := run.e.G.Obj(id).Tapped; got != tapped {
				t.Errorf("turn %s: %s tapped = %v, want %v", turn, ref, got, tapped)
			}
		}
	}
}

func TestOracleSetupTappedAfterUntapDropsTapTriggers(t *testing.T) {
	run, fails, transcript := tappedAfterUntapRun(t, `{"name":"taut-trig","setup":{"p0":{"battlefield":["Quest for Renewal","Grizzly Bears"],"tapped_after_untap":["Grizzly Bears"]}},"steps":[]}`)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	if !run.e.G.Obj(run.refs["p0:Grizzly Bears"]).Tapped {
		t.Fatal("precondition: Grizzly Bears is not tapped")
	}
	if n := len(run.e.G.Stack); n != 0 {
		t.Errorf("stack has %d objects, want 0", n)
	}
	// The drive stops at the first priority before queued triggers reach the
	// stack, so the queue is where a surviving Tap trigger would sit.
	if n := len(run.e.pendingTriggers); n != 0 {
		t.Errorf("%d triggers still queued after setup, want 0", n)
	}
	if n := run.e.G.Obj(run.refs["p0:Quest for Renewal"]).Counter("QUEST"); n != 0 {
		t.Errorf("Quest for Renewal has %d quest counters, want 0", n)
	}
}

func TestOracleSetupTappedAfterUntapRejects(t *testing.T) {
	cases := map[string]string{
		"prefix":      `{"battlefield":["Grizzly Bears"],"tapped_after_untap":["p0:Grizzly Bears"]}`,
		"hand only":   `{"hand":["Grizzly Bears"],"battlefield":["Forest"],"tapped_after_untap":["Grizzly Bears"]}`,
		"ordinal too": `{"battlefield":["Grizzly Bears","Grizzly Bears"],"tapped_after_untap":["Grizzly Bears#3"]}`,
		"empty":       `{"battlefield":["Grizzly Bears"],"tapped_after_untap":[""]}`,
	}
	for name, seat := range cases {
		_, fails, _ := tappedAfterUntapRun(t, `{"name":"taut-rej","setup":{"p0":`+seat+`},"steps":[]}`)
		if len(fails) != 1 || strings.Contains(fails[0], "panic") {
			t.Errorf("%s: fails = %v, want one harness error", name, fails)
		}
	}
}
