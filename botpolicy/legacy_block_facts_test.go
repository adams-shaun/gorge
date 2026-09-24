package botpolicy

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestLegacyDecideBlockNeverLoneBlocksATeamNeedingAttacker(t *testing.T) {
	const menace, bounded, plain = state.ObjID(10), state.ObjID(11), state.ObjID(12)
	d := decision.Decision{Seq: 1, Kind: decision.KBlockers, Min: 0, Max: 3}
	for i, attacker := range []state.ObjID{menace, bounded, plain} {
		for blocker := 1; blocker <= 3; blocker++ {
			o := decision.Option{Index: len(d.Options), Kind: "block", Obj: state.ObjID(blocker), Attacker: attacker}
			if i == 1 {
				o.MinBlockers = 3
			}
			d.Options = append(d.Options, o)
		}
	}
	if d.Options[0].Attacker != menace || d.Options[3].MinBlockers != 3 || d.Options[6].MinBlockers != 0 {
		t.Fatal("decision facts precondition failed")
	}
	plainPicked, first := false, []int(nil)
	for seed := uint64(0); seed < 250; seed++ {
		board := Board{Creatures: map[state.ObjID]Creature{menace: {Keywords: []string{"Menace"}}}}
		in := LegacyDecide(board, &d, rand.New(rand.NewPCG(seed, seed+1)))
		counts := map[state.ObjID]int{}
		for _, idx := range in.Choices {
			if idx >= 0 && idx < len(d.Options) {
				counts[d.Options[idx].Attacker]++
			}
		}
		if counts[menace] == 1 || counts[bounded] == 1 {
			t.Fatalf("seed %d lone team block: %v", seed, in.Choices)
		}
		plainPicked = plainPicked || counts[plain] > 0
		if seed < 4 {
			first = append(first, in.Choices...)
		}
	}
	if !plainPicked {
		t.Fatal("plain attacker was never selected")
	}
	if len(first) == 0 {
		t.Fatal("expected pinned choices")
	}
	want := []int{0, 1, 2, 0, 1, 2, 0, 1}
	if len(first) != len(want) {
		t.Fatalf("first choices %v, want %v", first, want)
	}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("first choices %v, want %v", first, want)
		}
	}
}
