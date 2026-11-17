package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tokenMemos applies the memory riders of one minted token o of in
// (OpCreateTokenEx B.N): the resolution's remembered set (gorge RememberTokens$),
// its imprinted set (ImprintTokens$, zone_imprint.go) and the token's own
// memory of its creating source (RememberSource$, kop.TokRememberSrc).
func (s *State) tokenMemos(in *kop.Instr, o Obj) {
	if in.B.N&kop.TokRememberObj != 0 {
		s.remember(o)
	}
	if in.B.N&kop.TokImprintObj != 0 {
		s.imprint(o) // zone_imprint.go
	}
	if uint16(in.B.N)&kop.TokRememberSrc != 0 && s.res.active && s.res.e.obj != NoObj {
		s.smAddAt(s.res.e.obj, o, smRem, s.objs[o].ver+1) // the entry that follows bumps ver (zones.go)
	}
}

// srcRemObj is the object token src remembers: the first live smRem record
// keyed by src (NoObj when none). The record dies with the token's
// incarnation (smLiveNow), as gorge's per-object Remembered list does.
func (s *State) srcRemObj(src Obj) Obj {
	if src == NoObj {
		return NoObj
	}
	r := coldSrcMem.of(s)
	for i := range r {
		if r[i].fl == 0 {
			break
		}
		if r[i].src == src && r[i].fl&smRem != 0 && s.smLiveNow(&r[i]) {
			if DebugSrcMemStats != nil {
				DebugSrcMemStats.Reads++
				DebugSrcMemStats.Matched++
			}
			return r[i].o
		}
	}
	return NoObj
}

// srcRemCounters is ExCountersOn under kop.CtrOnSrcRem: the counters of kind
// c on the object ctx's source remembers, read live on the battlefield.
func (s *State) srcRemCounters(c kop.CounterKind, ctx evalCtx) int {
	o := s.srcRemObj(ctx.src)
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return 0
	}
	return s.perms[s.objs[o].slot].ctrN(s, int(c))
}
