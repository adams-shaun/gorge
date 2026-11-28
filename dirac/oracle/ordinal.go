package oracle

import "fmt"

// ordinal.go compares the two engines' decision counters: gorge's
// decision.Decision.Ordinal (the decisions answered before the pending one)
// and the kernel's State.SeqNo (State.Decisions). Both search ports fold that
// count into their per-decision seed (azmcts / dirac/search DecisionSeed), so
// a searched decision mirrors only while the counters agree.
//
// The kernel counts one decision per gorge declaration (dirac
// decl_count.go). What it cannot count is a gorge answer beneath the
// canonical surface (Stats.Internal: a payment window, a mana colour ask,
// a lowering's activations, another seat's pass inside a script): the kernel
// autopays and poses none of them. The check nets those out, so it holds the
// kernel to every decision both engines pose. A lowering's last step is the
// play itself, the kernel's cast, so each completed lowering (Stats.Lowered)
// is one canonical answer inside Internal.

// seqNoSide is a kernel side that exposes its decision counter.
type seqNoSide interface {
	SeqNo() uint32
}

// SeqNo is the kernel State's decision counter.
func (r *realSide) SeqNo() uint32 { return r.s.SeqNo() }

// beneathSurface is the number of gorge answers the kernel never poses.
func (g *GorgeSide) beneathSurface() uint64 {
	return uint64(g.Stats.Internal - g.Stats.Lowered)
}

// OrdinalCheck is a RunConfig.Check: at every decision both sides agree on it
// reports an "ordinal" divergence when the gorge side's pending Ordinal, net
// of its answers beneath the canonical surface, is not the kernel's SeqNo. A
// kernel side with no counter is not compared. n, when set, counts the
// decisions compared.
func OrdinalCheck(n *int) func(ref *GorgeSide, k Side) *Divergence {
	return func(ref *GorgeSide, k Side) *Divergence {
		ks, ok := k.(seqNoSide)
		gd := ref.Pending()
		if !ok || gd == nil {
			return nil
		}
		if n != nil {
			*n++
		}
		under := ref.beneathSurface()
		if got := uint64(ks.SeqNo()); got+under != gd.Ordinal {
			return &Divergence{What: "ordinal", Field: string(gd.Kind),
				Detail: fmt.Sprintf("%s decision for p%d: gorge Ordinal %d (%d beneath the surface), kernel SeqNo %d", gd.Kind, gd.Player, gd.Ordinal, under, got)}
		}
		return nil
	}
}
