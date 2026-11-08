package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

var soundwaveCopiedPlay = `{"name":"soundwave-copied-play","cr":["707.12"],"why":"inline","setup":{"p0":{"battlefield":["Soundwave, Sonic Spy"],"tokens":[{"script":"r_1_1_goblin"}]},"p1":{"graveyard":["Lightning Bolt"]}},"steps":[` +
	`{"op":"attack","seat":0,"attackers":["p0:token:Goblin"],"defender":"p1"}` + passes(1) +
	`,{"op":"pass","seat":0},{"op":"pass","seat":1,"answers":[{"kind":"target","pick":["p1:Lightning Bolt"]}]},` +
	`{"op":"resolve","seat":0,"answers":[{"kind":"modes","pick":["Play Lightning Bolt"]},{"kind":"target","pick":["p1"]}]}` + passes(1) + `]}`

// TestSoundwaveCopiedPlayImprintsAndConverts: a CopyCard$ Play casts a minted
// copy while the original stays exiled, so ImprintPlayed$ must imprint the
// copy or Soundwave's Imprinted-gated DBConvert never runs.
func TestSoundwaveCopiedPlayImprintsAndConverts(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(soundwaveCopiedPlay))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if got := strings.Join(last.Players[1].Exile, ","); !strings.Contains(","+got+",", ",Lightning Bolt,") {
		t.Fatalf("p1 exile = %q, want the original Lightning Bolt still there", got)
	}
	if got := last.Players[1].Life; got != 16 {
		t.Fatalf("p1 life = %d, want 16 (the copied Bolt must have been cast)", got)
	}
	found := false
	for _, p := range last.Permanents {
		if p.Ref == "p0:Soundwave, Sonic Spy" {
			found = true
			if p.Name != "Soundwave, Superior Captain" {
				t.Fatalf("Soundwave name = %q, want it converted", p.Name)
			}
		}
	}
	if !found {
		t.Fatalf("no permanent with Ref p0:Soundwave, Sonic Spy")
	}
}
