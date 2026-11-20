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
		s.osAddAt(osRem, o, s.res.e.obj, s.objs[o].ver+1) // the token (src key) remembers its creating source; the entry that follows bumps ver (zones.go)
	}
}

// srcRemObj is the object token src remembers: the first live osRem record
// keyed by src (NoObj when none). The record dies with the token's
// incarnation (osLiveNow), as gorge's per-object Remembered list does.
func (s *State) srcRemObj(src Obj) Obj {
	if src == NoObj {
		return NoObj
	}
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src == src && rec.inKinds(osMRem) && s.osLiveNow(rec) {
			if DebugSrcMemStats != nil {
				DebugSrcMemStats.Reads++
				DebugSrcMemStats.Matched++
			}
			return rec.o
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
