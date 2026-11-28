package neteval

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestSameDecisionMemoSeesEveryOption: State.SeqNo counts a multi-pick
// declaration once, so two picks of one declaration can share the state, the
// sequence number, N and the first option: a must-attack declaration offers
// [A->p, M->p] and, once M is picked, [A->p, Done]. The same-decision memo
// must not answer the second pick from the first.
func TestSameDecisionMemoSeesEveryOption(t *testing.T) {
	var s dirac.State
	var n Net
	first := dirac.Decision{N: 2}
	first.Options[0], first.Options[1] = 1, 2
	n.remember(&s, &first, 0.5)
	again := first
	if !n.same(&s, &again) {
		t.Fatal("the remembered decision does not hit the memo")
	}
	next := first
	next.Options[1] = 3
	if next.N != first.N || next.Options[0] != first.Options[0] {
		t.Fatal("the two picks must agree on N and the first option")
	}
	if n.same(&s, &next) {
		t.Fatal("a pick offering other options hit the memo of the pick before it")
	}
}
