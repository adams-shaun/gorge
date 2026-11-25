package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectCommaTriggersFindsBecomeMonarch pins the one reader of an Effect's
// Triggers$ list that the effEffect split did not cover:
// effectNamesBecomeMonarchTrigger (rules' exile-until-monarch registration).
// Forge writes the list comma-separated, so a whole-list lookup misses a
// Mode$ BecomeMonarch body named after the comma (Palace Jailer's `ComeBack`
// is the shape) and the return promise is never armed.
func TestEffectCommaTriggersFindsBecomeMonarch(t *testing.T) {
	t.Parallel()
	h := &fakeHost{g: &state.Game{}}
	c := NewCtx(0, 0, CtxInit{SVars: map[string]string{
		"ComeBack": "Mode$ BecomeMonarch | ValidPlayer$ Opponent | Execute$ TrigReturn | Static$ True",
	}})
	sa := &cards.SA{API: "Effect", Params: map[string]string{"Triggers": "First,ComeBack"}}
	if !effectNamesBecomeMonarchTrigger(h, &c, sa) {
		t.Fatal("comma-separated Triggers$ did not find the BecomeMonarch body named second")
	}
}
