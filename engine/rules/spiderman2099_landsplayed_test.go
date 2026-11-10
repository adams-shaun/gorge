package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSpiderMan2099LandFromHand pins Spider-Man 2099's intervening-if
// (CheckSVar$ Y, Y = Count$ThisTurnCast_...!wasCastFromYourHand plus
// PlayerCountPropertyYou$LandsPlayed Card.!inRealZoneHand,Card.YouDontOwn):
// a land played from the hand does not satisfy it, so no end-step trigger;
// the same land played from the graveyard (Crucible of Worlds) does. The
// LandsPlayed read used to ignore its filter and count the hand play.
func TestSpiderMan2099LandFromHand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name, scenario string
		want           bool
	}{
		{"hand land does not trigger", `{"name":"x","cr":["603.4"],"why":"x","setup":{"p0":{"battlefield":["Spider-Man 2099"],"hand":["Forest"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"play","seat":0,"card":"p0:Forest"},{"op":"pass_to","seat":0,"step":"end","active":"p0"}]}`, false},
		{"graveyard land triggers", `{"name":"x","cr":["603.4"],"why":"x","setup":{"p0":{"battlefield":["Spider-Man 2099","Crucible of Worlds"],"graveyard":["Forest"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"play","seat":0,"card":"p0:Forest"},{"op":"pass_to","seat":0,"step":"end","active":"p0"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := RunOracleScenarioJSON(reg, []byte(tc.scenario))
			if err != nil || len(res.Fails) != 0 {
				t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			if last.Step != "end" || last.Active != 0 {
				t.Fatalf("precondition: last checkpoint at step %q active %d, want p0's end step", last.Step, last.Active)
			}
			played := false
			for _, p := range last.Permanents {
				played = played || p.Name == "Forest"
			}
			if !played {
				t.Fatal("precondition: the Forest was not played onto the battlefield")
			}
			got := false
			for _, s := range last.Stack {
				got = got || (s.Kind == "ability" && strings.HasSuffix(s.Source, "Spider-Man 2099"))
			}
			if got != tc.want {
				t.Fatalf("Spider-Man 2099 end-step trigger = %v, want %v\nsnapshots=%+v\ndecisions=%+v", got, tc.want, res.Snapshots, res.Decisions)
			}
		})
	}
}
