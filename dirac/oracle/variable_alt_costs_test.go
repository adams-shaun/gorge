package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestVariableAltCostReprosReach: the two repros reach the shapes they pin.
// Broodguard Elite's warp cast announces an X above 0 (and once at 0), and
// Fugitive Codebreaker is turned face up; the lockstep rows alone would
// accept a script that never got there.
func TestVariableAltCostReprosReach(t *testing.T) {
	t.Parallel()
	read := func(name string) *Repro {
		r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	warp := read("warp_x_broodguard_elite")
	code, ok := chooseCode("x")
	if !ok {
		t.Fatal("no choose:x code")
	}
	var zero, above int
	for i, a := range warp.Actions {
		if a.Kind() != ActCast || a.Mode()&0x0f != 8 || i+1 >= len(warp.Actions) {
			continue
		}
		if x := warp.Actions[i+1]; x.Kind() == ActChoose+CanonKind(code) {
			if x.Mode() == 0 {
				zero++
			} else {
				above++
			}
		}
	}
	if zero == 0 || above == 0 {
		t.Errorf("warp casts announcing X = 0: %d, above 0: %d; want both", zero, above)
	}
	up := 0
	for _, a := range read("disguise_reduced_turn_up_fugitive_codebreaker").Actions {
		if a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp {
			up++
		}
	}
	if up == 0 {
		t.Error("the Codebreaker repro never turns a permanent face up")
	}
}

// TestDisguiseReductionMutation: without the printed reduction the kernel
// prices Fugitive Codebreaker's turn-up at {5}{R}, so the floating {R}{R}{R}
// no longer offers it where gorge does.
func TestDisguiseReductionMutation(t *testing.T) {
	dirac.DebugNoMorphReduce = true
	defer func() { dirac.DebugNoMorphReduce = false }()
	if div := replayStdRepro(t, synthReg(t), "custom", "disguise_reduced_turn_up_fugitive_codebreaker", false); div == nil {
		t.Fatal("the mutation did not diverge")
	}
}
