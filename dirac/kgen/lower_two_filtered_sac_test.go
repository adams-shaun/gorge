package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoFilteredSacrificePartsRefuse: a cost with two filtered sacrifice parts
// (Jarad, Golgari Lich Lord's "Sac<1/Swamp> Sac<1/Forest>") has one sacrifice
// slot in the lowered kop.Cost, so the second part used to overwrite the first
// and the kernel offered the return with no Swamp to sacrifice, which gorge's
// cost payer never does (CR 118.3, 602.2b). A printed battlefield activated
// ability's parts now lower as a cost program, one Sac atom per part in script
// order (lower_costprog.go); beside a mana cost they stay refused (the
// planner's sacrifice reservation claims one part). A lone filtered part
// still lowers without a program.
func TestTwoFilteredSacrificePartsRefuse(t *testing.T) {
	const act = "Name:Sac Probe\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ %s | LifeAmount$ 1\n"
	build := func(cost string) (kop.Ability, string, [][]kop.CostAtom) {
		d, rs, progs := costProgBuild(t, "Sac Probe", strings.Replace(act, "%s", cost, 1))
		var a kop.Ability
		if len(d.Abilities) > 0 {
			a = d.Abilities[0]
		}
		return a, kpaylifexReasons(rs), progs
	}
	for _, cost := range []string{"Sac<1/Swamp> Sac<1/Forest>", "Sac<1/Forest> Sac<1/Swamp>"} {
		// the land subtypes need the corpus's subtype table: lowering only
		if got := strings.Join(lowerScript(t, "Sac Probe", strings.Replace(act, "%s", cost, 1)), "; "); got != "" {
			t.Errorf("Cost$ %s: lowering refused %q", cost, got)
		}
	}
	if a, got, _ := build("Sac<1/Artifact>"); got != "" || a.CostProg != 0 {
		t.Errorf("a lone Sac<1/Artifact>: refused %q, CostProg %d", got, a.CostProg)
	}
	for _, cost := range []string{"Sac<1/Creature> Sac<1/Artifact>", "Sac<1/Artifact> Sac<1/Creature>", "T Sac<1/Creature> Sac<1/Land>"} {
		a, got, progs := build(cost)
		if got != "" || a.CostProg == 0 {
			t.Errorf("Cost$ %s: refused %q, CostProg %d", cost, got, a.CostProg)
			continue
		}
		n := 0
		for _, at := range progs[a.CostProg] {
			if at.K == kop.CostAtomSac && at.F != 0 && at.N == kop.Const(1) {
				n++
			}
		}
		if n != 2 {
			t.Errorf("Cost$ %s: program %+v, want two filtered Sac atoms", cost, progs[a.CostProg])
		}
	}
	if _, got, _ := build("1 Sac<1/Creature> Sac<1/Land>"); !strings.Contains(got, "beside a second sacrifice part") {
		t.Errorf("Cost$ 1 Sac<1/Creature> Sac<1/Land>: refusals %q, want the mana refusal", got)
	}
}
