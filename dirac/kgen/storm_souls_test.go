package kgen

import (
	"strings"
	"testing"
)

// TestStormSoulsSweep pins a ChangeZoneAll sweep out
// of a graveyard onto the battlefield reads StaticEffect$ as a per-entrant
// rider (kop.MvEntryRiders), the same as Ghost Vacuum's exile sweep.
func TestStormSoulsSweep(t *testing.T) {
	cs := lowerNamed(t, []string{"Storm of Souls"})
	if len(cs) != 1 || cs[0].Name != "Storm of Souls" {
		t.Fatalf("lowered %d cards, want Storm of Souls", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("Storm of Souls refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	lir := sb.String()
	for _, want := range []string{"ChangeZoneAll", "from=Graveyard", "static=", "override{pt=1/1 addsub=Spirit kw=Flying}"} {
		if !strings.Contains(lir, want) {
			t.Errorf("Storm of Souls LIR lacks %q:\n%s", want, lir)
		}
	}
	for _, l := range strings.Split(lir, "\n") {
		if strings.Contains(l, "ChangeZoneAll") {
			t.Log(strings.TrimSpace(l))
		}
	}
}
