package dirac

// DebugNoCardBasePower is the mutation switch of TestDemoCloneAddMutation:
// ExCardBasePower reads 0.
var DebugNoCardBasePower bool

// cardBasePower is kop.ExCardBasePower (Count$CardBasePower, gorge
// count_body_paid.go: h.Chars(Source).BasePower): the evaluating source's base
// power, the layer-7b value before counters and pumps (CR 613.4), a copy's
// copied power among it. A source off the battlefield reads its printed power,
// as gorge's Chars falls back to the face.
//
//go:noinline
func (s *State) cardBasePower(k *prep, ctx evalCtx) int {
	if DebugCloneCardStats != nil {
		DebugCloneCardStats.Base++
	}
	if DebugNoCardBasePower {
		return 0
	}
	if ctx.srcSl != NoSlot {
		bp, _ := s.basePTGen(k, ctx.srcSl)
		return bp
	}
	if ctx.src == NoObj {
		return 0
	}
	return int(k.cards[s.objs[ctx.src].card].power)
}
