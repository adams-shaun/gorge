package payexec

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// itlimocWheel is the stage-1 wheel Itlimoc, Cradle of the Sun poses: its
// plain {T}: Add {G} and its Amount$ X "Add {G} for each creature you
// control", both labelled "Add G" (rules' manaAmountPips keeps a non-literal
// amount as the bare pip).
func itlimocWheel() *decision.Decision {
	return &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "mana", Obj: 7, Ability: 0, Label: "Add G"},
		{Index: 1, Kind: "mana", Obj: 7, Ability: 1, Label: "Add G"},
	}}
}

// TestTiedManaWheelProvesTheWitness: a witness tapping Itlimoc for GGGGG
// takes the option the prove hook shows makes GGGGG, not the first "Add G".
func TestTiedManaWheelProvesTheWitness(t *testing.T) {
	act := decision.PaymentActivation{Source: 7, Produces: decision.ManaAmount{4: 5}}
	d := itlimocWheel()
	// Precondition: the labels alone tie, and the class walk leads with
	// option 0 (the plain {G}), which makes the wrong mana.
	if got, ok := matchMana(d, act); !ok || len(got) != 1 || got[0] != 0 {
		t.Fatalf("precondition: label-only pick %v (ok %v), want [0]", got, ok)
	}
	var tried []int
	prove := func(pd *decision.Decision, pa decision.PaymentActivation, choices []int) bool {
		if pd != d || pa.Produces != act.Produces || len(choices) != 1 {
			t.Fatalf("prove called with %v %v %v", pd, pa, choices)
		}
		tried = append(tried, choices[0])
		return choices[0] == 1
	}
	got, ok := matchManaProved(d, act, prove)
	if !ok || len(got) != 1 || got[0] != 1 {
		t.Fatalf("proved pick %v (ok %v), want [1]", got, ok)
	}
	if len(tried) != 2 || tried[0] != 0 || tried[1] != 1 {
		t.Fatalf("prove tried %v, want the leading option first then the other: [0 1]", tried)
	}
	// No member proven: the leading option stands (the pool check catches it).
	if got, ok := matchManaProved(d, act, func(*decision.Decision, decision.PaymentActivation, []int) bool { return false }); !ok || got[0] != 0 {
		t.Fatalf("unproven pick %v (ok %v), want [0]", got, ok)
	}
}

// TestUntiedManaWheelNeverProves: a wheel whose labels settle the witness
// (a dual's "Add U" beside "Add R") never reaches the prove hook.
func TestUntiedManaWheelNeverProves(t *testing.T) {
	d := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "mana", Obj: 7, Ability: 0, Label: "Add U", ManaSymbol: "U"},
		{Index: 1, Kind: "mana", Obj: 7, Ability: 1, Label: "Add R", ManaSymbol: "R"},
	}}
	act := decision.PaymentActivation{Source: 7, Produces: decision.ManaAmount{3: 1}}
	got, ok := matchManaProved(d, act, func(*decision.Decision, decision.PaymentActivation, []int) bool {
		t.Fatal("prove called for an untied wheel")
		return false
	})
	if !ok || len(got) != 1 || got[0] != 1 {
		t.Fatalf("pick %v (ok %v), want [1]", got, ok)
	}
}
