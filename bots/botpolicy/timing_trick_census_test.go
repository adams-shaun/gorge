package botpolicy_test

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/bots/spellbench"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestTrickCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	universe, err := spellbench.FDNCards()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"Claws Out":           true,
		"Divine Resilience":   true,
		"Fake Your Own Death": true,
		"Fleeting Flight":     true,
		"Giant Growth":        true,
		"Snakeskin Veil":      true,
		"Sure Strike":         true,
	}
	var got []string
	for _, u := range universe {
		card, ok := reg.Lookup(u.Name)
		if !ok {
			t.Fatalf("FDN card %q missing from corpus", u.Name)
		}
		face := card.Faces[0]
		if !face.IsInstant() {
			continue
		}
		if effects.CastCombatTrick(face.SpellAbility()) {
			got = append(got, u.Name)
			if !want[u.Name] {
				t.Errorf("unexpected combat trick %q", u.Name)
			}
		}
	}
	sort.Strings(got)
	t.Logf("FDN combat tricks: %v", got)
	var wantNames []string
	for name := range want {
		wantNames = append(wantNames, name)
	}
	sort.Strings(wantNames)
	if len(got) != len(wantNames) {
		t.Fatalf("combat tricks = %v, want %v", got, wantNames)
	}
	for i := range wantNames {
		if got[i] != wantNames[i] {
			t.Fatalf("combat tricks = %v, want %v", got, wantNames)
		}
	}
}
