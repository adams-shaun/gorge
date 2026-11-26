package kgen

// Nimble Hobbit is a triggered controller api:GenericChoice whose every
// choice carries an UnlessCost$. The trigger's choice is asked at resolution
// (CR 608.2d; engine/rules trigger_queue.go), so kgen lowers it like a
// spell's: an OpChooseBranch C 0 with one Branch per choice, each body
// opening with its UnlessCost$ (OpUnlessSwitched), so branchPayable drops a
// branch whose cost the controller cannot pay at the ask (gorge effCharm's
// GenericChoiceAvailable; dirac/interp_ask.go branchPayable).

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestNimbleHobbitTriggerUnlessGateAtResolution(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Nimble Hobbit"})
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want one Nimble Hobbit", len(lw.cards))
	}
	if n := len(lw.cards[0].Refusals); n != 0 {
		t.Fatalf("Nimble Hobbit refused: %s", lw.cards[0].Refusals[0].Reason)
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 {
		t.Fatalf("built %d cards, want one", len(bs))
	}
	found := false
	for i := range bs[0].def.Abilities {
		ab := &bs[0].def.Abilities[i]
		if ab.AFlags&kop.AfCharmTrigger != 0 {
			t.Fatalf("ability %d carries the retired placement charm mark (kop.AfCharmTrigger)", i)
		}
		for j := range ab.Prog {
			if ab.Prog[j].Op != kop.OpChooseBranch || ab.Prog[j].C != 0 {
				continue
			}
			found = true
			branches := 0
			for k := j + 1; k < len(ab.Prog); {
				if ab.Prog[k].Op != kop.OpBranch {
					k++
					continue
				}
				branches++
				if k+1 >= len(ab.Prog) || ab.Prog[k+1].Op != kop.OpUnlessSwitched {
					t.Fatalf("branch %d of the resolution ask opens with %v, want its UnlessCost$ (OpUnlessSwitched)", ab.Prog[k].C, ab.Prog[k+1].Op)
				}
				k += 1 + int(ab.Prog[k].A.N)
			}
			if branches != 2 {
				t.Fatalf("the resolution ask has %d branches, want Nimble Hobbit's two unless-cost choices", branches)
			}
		}
	}
	if !found {
		t.Fatal("no OpChooseBranch C 0 (the unless-gated resolution ask) in the built Nimble Hobbit")
	}
}
