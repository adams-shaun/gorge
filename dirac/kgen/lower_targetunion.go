package kgen

import (
	"fmt"
	"os"
)

// union: Defined$ Targeted over several target slots. gorge binds the
// resolution's Ctx.AllTargets to the chain's target union (rules
// trigger_subtargets.go chainTargetUnion: the root's targets, then each
// announced SubAbility$ link's rechecked answers, in chain order) and
// Defined$ Targeted reads it (effects count_ref.go refTargetUnion). A
// target found illegal on resolution is dropped from the union (CR 608.2b:
// recheckCastSubTargets / resolutionTargets), so it is not affected.
//
// kgen lowers an effect subject Defined$ Targeted in a chain with more than
// one slot as rTargetUnion over the chain's slots: the kernel runs the op
// once per legal object of those slots, in slot order (kernel
// tgt_union.go). Exact only when
//   - the read is in the ability's own linear chain (or a charm mode's): a
//     nested branch's slots are not chain links in gorge;
//   - every slot is announced (none is chosen mid-resolution: gorge's union
//     holds only castSubTargets);
//   - the slots are every slot of the ability (of the mode), so no later
//     link's slot is missing from the union (unionSlotsWhole);
//   - the op is one whose per-object run is gorge's per-object loop
//     (unionOps).

// unionOps are the ops a union subject is lowered for.
var unionOps = map[string]bool{"Destroy": true, "GrantKeyword": true, "Return": true, "Override": true, "Exile": true}

// targetUnion is an effect subject's Defined$ Targeted over len(chainT) > 1
// slots.
func (lc *lowerCtx) targetUnion() (lRef, error) {
	lc.unionProbe("defined:Targeted")
	if !(lc.depth == 1 && lc.mode < 0 || lc.depth == 2 && lc.mode >= 0) {
		return lRef{}, refuse("Defined$ Targeted over %d target slots in a nested chain", len(lc.chainT))
	}
	m := 0
	for _, t := range lc.chainT {
		if lc.ab.Targets[t].AtRes {
			return lRef{}, refuse("Defined$ Targeted over a mid-resolution target slot")
		}
		if t > 30 {
			return lRef{}, refuse("Defined$ Targeted over target slot %d", t)
		}
		m |= 1 << t
	}
	return lRef{Kind: rTargetUnion, N: m}, nil
}

// unionUses refuses a union ref the instructions from at on use outside an
// op's subject or in an op not in unionOps.
func (lc *lowerCtx) unionUses(at int) error {
	for _, in := range lc.ab.Prog[at:] {
		if in.Obj2.Kind == rTargetUnion || in.A.Ref.Kind == rTargetUnion || in.B.Ref.Kind == rTargetUnion {
			return refuse("Defined$ Targeted target union outside an op's subject")
		}
		if in.Obj.Kind == rTargetUnion && !unionOps[in.Op] {
			return refuse("Defined$ Targeted target union for op %s", in.Op)
		}
	}
	return nil
}

// unionSlotsWhole refuses a union whose slots are not every slot of its
// mode (a later link's or a nested chain's slot gorge would or would not
// read), once the ability's whole chain is lowered.
func (lc *lowerCtx) unionSlotsWhole() error {
	for _, in := range lc.ab.Prog {
		if in.Obj.Kind != rTargetUnion {
			continue
		}
		mode, all := -2, 0
		for i, t := range lc.ab.Targets {
			if in.Obj.N&(1<<i) != 0 {
				mode = t.Mode
			}
		}
		for i, t := range lc.ab.Targets {
			if t.Mode == mode {
				all |= 1 << i
			}
		}
		if all != in.Obj.N {
			return refuse("Defined$ Targeted target union over slots %s of %s", in.Obj, lRef{Kind: rTargetUnion, N: all})
		}
	}
	return nil
}

// unionProbe is the union census: a Targeted read in a chain that has
// declared more than one target slot (KGEN_UNION_PROBE=1, stderr).
func (lc *lowerCtx) unionProbe(site string) {
	if len(lc.chainT) > 1 && os.Getenv("KGEN_UNION_PROBE") != "" {
		fmt.Fprintf(os.Stderr, "UNIONPROBE\t%s\t%s\tslots=%v\tmode=%d\tface=%d\n", lc.name, site, lc.chainT, lc.mode, lc.out.FaceMode)
	}
}
