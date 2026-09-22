package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

func TestRepeatOptionalRecordsCursorWhenBodySuspends(t *testing.T) {
	Register("TestRepeatOptionalSuspendingBody", func(Host, *Ctx, *cards.SA) {})
	t.Cleanup(func() { unregister("TestRepeatOptionalSuspendingBody") })

	h, c := fixtureHost(t)
	h.suspendAfterAsk = true
	c.SVars = map[string]string{"Body": "DB$ TestRepeatOptionalSuspendingBody"}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{
		"RepeatSubAbility": "Body", "RepeatOptional": "True",
	}})
	if !h.repeatOptionalCalled {
		t.Fatal("RepeatOptional body suspension did not register a continuation")
	}
	if h.repeatOptionalNext != 1 {
		t.Fatalf("RepeatOptional cursor = %d, want 1", h.repeatOptionalNext)
	}
}
