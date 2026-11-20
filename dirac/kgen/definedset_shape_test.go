package kgen

import (
	"strings"
	"testing"
)

// TestDefinedSetKopShape: a RefSet instruction builds only on a kop op the
// kernel's execSet serves, in a shape it serves. Wall of Nets's end-of-combat
// "Defined$ Valid Creature.blockedBySource" Exile remembers what it exiles,
// which lowers to OpExileEx: execSet panicked on it in lockstep play, so the
// build refuses. The plain Exile beside it still builds.
func TestDefinedSetKopShape(t *testing.T) {
	remembering := ksetCard(t, "Sorcery", "SP$ ChangeZone | Defined$ Valid Creature.YouCtrl | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True", "", map[string]string{}, nil)
	if len(remembering.Refusals) != 0 {
		t.Fatalf("LIR refused, so the build check is not reached: %+v", remembering.Refusals)
	}
	if got := ksetBuildRefusal(t, remembering); !strings.Contains(got, "shape") {
		t.Fatalf("a remembering Exile over a set must refuse on its kop shape, got %q", got)
	}
	plain := ksetCard(t, "Sorcery", "SP$ ChangeZone | Defined$ Valid Creature.YouCtrl | Origin$ Battlefield | Destination$ Exile", "", map[string]string{}, nil)
	if got := ksetBuildRefusal(t, plain); got != "" {
		t.Fatalf("a plain Exile over a set should build, got %q", got)
	}
}
