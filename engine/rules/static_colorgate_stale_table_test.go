package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// FDN-s123: Witness Protection resolves onto Three Tree Mascot, making it
// green and white, so Knight of Malice's "+1/+0 as long as any player controls
// a white permanent" holds (CR 613.1e, 604.2). The minimal board that missed it
// needs Imprisoned in the Moon on Erudite Wizard beside the Knight and the
// Mascot: a colour effect already live before the Aura arrives. The rescan at
// the Aura's entry read the layer-5 colour table off the previous static list
// (the Wizard colourless, the Mascot still green) and cached it at that log
// head, and the colour gate's second walk was served the same stale table, so
// the Knight stayed 2/2. Without Imprisoned in the Moon the old list had no
// colour effect and nothing was cached, which is why the three-permanent probe
// read 3/2.
func TestKnightOfMaliceSeesRecolouredPermanent(t *testing.T) {
	t.Parallel()
	const scn = `{"name": "FDN-s123 Knight of Malice", "turn": 1, "cr": ["613.1e", "604.2"], "why": "x",
 "setup": {
  "p0": {"battlefield": ["Knight of Malice", "Island", "Imprisoned in the Moon"], "hand": ["Witness Protection"],
         "library_top": ["Island", "Island", "Island", "Island", "Island"], "life": 20,
         "attached": [{"card": "Imprisoned in the Moon", "to": "p1:Erudite Wizard"}]},
  "p1": {"battlefield": ["Three Tree Mascot", "Erudite Wizard"],
         "library_top": ["Island", "Island", "Island", "Island", "Island"], "life": 20}},
 "steps": [
  {"op": "activate", "seat": 0, "card": "p0:Island", "ability_index": 0},
  {"op": "cast", "seat": 0, "card": "p0:Witness Protection", "targets": ["p1:Three Tree Mascot"]},
  {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scn))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	if len(res.Snapshots) == 0 {
		t.Fatal("no snapshots")
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if len(last.Stack) != 0 {
		t.Fatalf("Witness Protection has not resolved: stack %v", last.Stack)
	}
	perm := map[string]OracleSnapPerm{}
	for _, p := range last.Permanents {
		perm[p.Ref] = p
	}
	// Preconditions: the Aura sits on the Mascot, which is now green and
	// white, and Imprisoned in the Moon's colour effect is live on the Wizard.
	if got := perm["p0:Witness Protection"].AttachedTo; got != "p1:Three Tree Mascot" {
		t.Fatalf("Witness Protection attached to %q, want the Mascot", got)
	}
	if got := perm["p1:Three Tree Mascot"].Colors; got != "WG" {
		t.Fatalf("Mascot colours %q, want WG", got)
	}
	if got := perm["p0:Imprisoned in the Moon"].AttachedTo; got != "p1:Erudite Wizard" {
		t.Fatalf("Imprisoned in the Moon attached to %q, want the Wizard", got)
	}
	if w, ok := perm["p1:Erudite Wizard"]; !ok || w.Colors != "" {
		t.Fatalf("Erudite Wizard %+v, want a colourless land", w)
	}
	if got := perm["p0:Knight of Malice"].PT; got != "3/2" {
		t.Fatalf("Knight of Malice is %s, want 3/2 beside a white permanent", got)
	}
}
