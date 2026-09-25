package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
)

func TestStoreSVarExpressionSVarPlusOneResolves(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"Aid": "Number$0"}
	if body := c.SVars["Aid"]; body != "Number$0" {
		t.Fatalf("printed Aid body = %q, want Number$0", body)
	}
	if v, ok := runtimeSVar(c, "Aid"); ok {
		t.Fatalf("precondition: runtime Aid already present = %d", v)
	}
	store := sa(t, "DB$ StoreSVar | SVar$ Aid | Type$ CountSVar | Expression$ Aid/Plus.1")
	for want := int32(1); want <= 2; want++ {
		Resolve(h, c, store)
		for _, e := range h.log {
			if e.Kind == events.Note && strings.Contains(e.Text, "not resolvable") {
				t.Fatalf("unexpected unresolved note: %q", e.Text)
			}
		}
		if got, ok := runtimeSVar(c, "Aid"); !ok || got != want {
			t.Fatalf("runtime Aid = %d, present=%v; want %d", got, ok, want)
		}
		h.log = nil
	}
}

func TestStoreSVarExpressionUnmodelledOperandStaysLoud(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"Aid": "Number$0"}
	Resolve(h, c, sa(t, "DB$ StoreSVar | SVar$ Aid | Type$ CountSVar | Expression$ Aid/Plus.X"))
	found := false
	for _, e := range h.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "not resolvable") {
			found = true
		}
	}
	if !found {
		t.Fatal("unmodelled operand did not emit not-resolvable Note")
	}
	if got, ok := runtimeSVar(c, "Aid"); ok {
		t.Fatalf("unmodelled operand stored Aid=%d, want no write", got)
	}
}
