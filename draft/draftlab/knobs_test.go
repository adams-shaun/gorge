package draftlab

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// TestKnobsRefCarriesHash: a knobs:<path> ref resolves the file's knobs
// for both roles and records knobs@<hash>; a stale hash and an unknown key
// are refused.
func TestKnobsRefCarriesHash(t *testing.T) {
	l := testLab(t)
	path := filepath.Join(t.TempDir(), "fdn.json")
	if err := os.WriteFile(path, []byte(`{"colour_fit_max": 8, "signal_weight": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	want := pickbot.Tuning{ColourFitMax: 8, SignalWeight: 2}.Hash()
	ref, err := ParseAgent(KnobsPrefix + path)
	if err != nil {
		t.Fatal(err)
	}
	nd, err := l.Drafter(ref)
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	if nd.Ref.String() != "knobs@"+want {
		t.Fatalf("drafter ref = %s, want knobs@%s", nd.Ref, want)
	}
	if k, ok := nd.Drafter.(*pickbot.Knobbed); !ok || k.Tuning.SignalWeight != 2 {
		t.Fatalf("drafter = %#v, want a Knobbed with the file's knobs", nd.Drafter)
	}
	nb, err := l.Builder(ref)
	if err != nil {
		t.Fatalf("Builder: %v", err)
	}
	if _, ok := nb.Builder.(*pickbot.Best23); !ok || nb.Ref.String() != "knobs@"+want {
		t.Fatalf("builder = %T %s, want Best23 knobs@%s", nb.Builder, nb.Ref, want)
	}
	if _, err := l.Drafter(AgentRef{Name: ref.Name, Version: "000000000000"}); !errors.Is(err, ErrVersion) {
		t.Fatalf("stale hash: err = %v, want ErrVersion", err)
	}
	if err := os.WriteFile(path, []byte(`{"cfm": 8}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Builder(ref); err == nil || !strings.Contains(err.Error(), `"cfm"`) {
		t.Fatalf("unknown key: err = %v, want one naming cfm", err)
	}
}
