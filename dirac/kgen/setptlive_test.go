package kgen

import (
	"strings"
	"testing"
)

// TestSetPTStaticLiveLowers: an attached static whose SetPower$/SetToughness$
// is not a decimal integer lowers as a live layer-7b set anchored on the
// static's source (FIN Aettir and Priwen, whose value is its controller's
// life total). The per-affected word (AffectedX) is a different anchor and
// stays refused by name.
func TestSetPTStaticLiveLowers(t *testing.T) {
	found := map[string]*lCard{}
	for _, c := range lowerNamed(t, []string{"Aettir and Priwen", "Animate Artifact"}) {
		found[c.Name] = c
	}
	c := found["Aettir and Priwen"]
	if c == nil {
		t.Fatal("Aettir and Priwen: not lowered")
	}
	for _, r := range c.Refusals {
		t.Errorf("Aettir and Priwen: refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	if want := "override{livept pt=LifeOf(You)/LifeOf(You)}"; !strings.Contains(sb.String(), want) {
		t.Errorf("Aettir and Priwen: lowered IR lacks %q:\n%s", want, sb.String())
	}

	c = found["Animate Artifact"]
	if c == nil {
		t.Fatal("Animate Artifact: not lowered")
	}
	refused := false
	for _, r := range c.Refusals {
		refused = refused || strings.Contains(r.Reason, "AffectedX")
	}
	if !refused {
		t.Errorf("Animate Artifact: refusals %v, want one naming AffectedX", c.Refusals)
	}
}
