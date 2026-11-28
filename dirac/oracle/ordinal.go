package oracle

import "fmt"

// ordinal.go compares the two engines' decision counters: gorge's
// decision.Decision.Ordinal (the decisions answered before the pending one)
// and the kernel's State.SeqNo (State.Decisions). Both search ports fold that
// count into their per-decision seed (azmcts / dirac/search DecisionSeed), so
// a searched decision mirrors only while the counters agree.

// seqNoSide is a kernel side that exposes its decision counter.
type seqNoSide interface {
	SeqNo() uint32
}

// SeqNo is the kernel State's decision counter.
func (r *realSide) SeqNo() uint32 { return r.s.SeqNo() }

// OrdinalCheck is a RunConfig.Check: at every decision both sides agree on it
// reports an "ordinal" divergence when the gorge side's pending Ordinal is not
// the kernel's SeqNo. A kernel side with no counter is not compared. n, when
// set, counts the decisions compared.
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
		if got := uint64(ks.SeqNo()); got != gd.Ordinal {
			return &Divergence{What: "ordinal", Field: string(gd.Kind),
				Detail: fmt.Sprintf("%s decision for p%d: gorge Ordinal %d, kernel SeqNo %d", gd.Kind, gd.Player, gd.Ordinal, got)}
		}
		return nil
	}
}
