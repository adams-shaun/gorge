package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StoreSVar's per-object number (gorge effStoreSVar: events.StoreSVar folds the
// resolved amount into Object.RuntimeSVars under the SVar name, last write
// wins; every reader checks that store before the printed SVar, effects/
// count.go runtimeSVar; the store is cleared when the object leaves the
// battlefield, events/apply_zone.go, CR 400.7).
//
// The number is a record of kind osNum in the object-set store
// (zone_objsets.go): src the carrier, o the value as an int16, kind's high
// nibble the SVar's index in Table.SVarNames (at most kop.15). It
// mirrors gorge's one rule and nothing finer: the number belongs to the object,
// whatever zone it is in, and only a departure from the battlefield clears it
// (numClear, from removePerm). So a write after the source has left (a
// stacked ability resolving after its source died) persists on the departed
// object and is read back by the next ability that resolves, and re-entering
// the battlefield from another zone clears nothing, exactly as gorge's
// foldStoreSVar and wasBattlefield reset do. There is no incarnation stamp
// and no look-back: sver is unused, and osGC never collects a number.
//
//   - ExtStoreNum writes the resolving source's record. A write of the printed
//     default (a literal the writer's card prints, kop.StoreNumC) deletes the
//     record, so the store holds only the numbers that differ.
//   - ExSVarNum reads it: the record, else the printed SVar body (Expr.Sub[0]).
//
// A value past the int16 range saturates and stalls StallObjSets, as does a
// write that finds the store full. A token off the battlefield takes no write
// (it ceases at the next state-based check, before anything could read it, and
// its slot is reused).

// useStoreNum is genPrep.uses' bit for a table whose cards write a number:
// objSetsCap gives its object-set store room.
var useStoreNum = newGenUse(func(t *kop.Table) bool { return len(t.SVarNames) > 0 })

// DebugNumStoreNoWrite (TestNumStoreMutation): ExtStoreNum writes nothing.
var DebugNumStoreNoWrite bool

// NumStoreStats counts the store's lockstep coverage.
type NumStoreStats struct {
	Writes    int // records written or overwritten
	Deletes   int // a write of the printed default dropped its record
	Dropped   int // a write from a token that had left the battlefield
	Cleared   int // numbers dropped by a departure from the battlefield
	Late      int // writes whose source had already left the battlefield (a stacked ability outliving it)
	Saturated int // a value past int16
	Reads     int // reads
	Hits      int // reads that found a live record
}

// DebugNumStoreStats, when non-nil, accumulates the counters above.
var DebugNumStoreStats *NumStoreStats

// numTag is the kind byte of name's record.
func numTag(name int) uint8 { return osNum | uint8(name)<<4 }

// numStore writes src's number for name: v, or nothing stored when del and v
// is the printed default def.
func (s *State) numStore(src Obj, name, v int, del bool, def int) {
	st := DebugNumStoreStats
	if DebugNumStoreNoWrite || src == NoObj {
		return
	}
	ob := &s.objs[src]
	if ob.of&ofToken != 0 && ob.zone != kop.ZoneBattlefield {
		if st != nil {
			st.Dropped++
		}
		return
	}
	if st != nil && ob.zone != kop.ZoneBattlefield {
		st.Late++
	}
	if v > 32767 || v < -32768 {
		v = max(-32768, min(32767, v))
		if st != nil {
			st.Saturated++
		}
		s.capFull(StallObjSets)
	}
	tag := numTag(name)
	r := s.cold.objSets
	n := 0
	for ; n < len(r) && r[n].kind != 0; n++ {
		if r[n].src != src || r[n].kind != tag {
			continue
		}
		if del && v == def {
			s.osDrop(func(x *osRec) bool { return x.src == src && x.kind == tag })
			if st != nil {
				st.Deletes++
			}
			return
		}
		r[n].o = Obj(uint16(int16(v)))
		if st != nil {
			st.Writes++
		}
		return
	}
	if del && v == def {
		return // the printed default is what an absent record reads
	}
	if n >= len(r) {
		s.osGC()
		if n = s.osLen(); n >= len(r) {
			s.capFull(StallObjSets)
			return
		}
	}
	r[n] = osRec{o: Obj(uint16(int16(v))), src: src, kind: tag}
	if st != nil {
		st.Writes++
	}
}

// numRead is src's stored number for name.
func (s *State) numRead(src Obj, name int) (int, bool) {
	if src == NoObj {
		return 0, false
	}
	tag := numTag(name)
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			return 0, false
		}
		if r[i].src == src && r[i].kind == tag {
			return int(int16(uint16(r[i].o))), true
		}
	}
	return 0, false
}

// exSVarNum evaluates ExSVarNum: the evaluating source's stored number for
// the name x.N, else the printed body in Sub[0].
func (s *State) exSVarNum(k *prep, x *kop.Expr, ctx evalCtx) int {
	st := DebugNumStoreStats
	if st != nil {
		st.Reads++
	}
	if v, ok := s.numRead(ctx.src, int(x.N)); ok {
		if st != nil {
			st.Hits++
		}
		return v
	}
	return s.evalV(k, x.Sub[0], ctx)
}

// storeNumOp is ExtStoreNum: B the value, C the packed name and printed
// default (kop.StoreNumC); the target is the resolving source.
//
//kmanifest:ext ExtStoreNum
func (s *State) storeNumOp(k *prep, in *kop.Instr) (ok, done bool) {
	if DebugOptInNoAccum && s.g.repEnd != 0 && s.res.pc >= s.g.repPC && s.res.pc < s.g.repEnd {
		return true, true // the mutation: an opt-in loop's accumulator never moves (act_optinseat.go)
	}
	name, del, def := kop.StoreNumUnpack(in.C)
	src := s.res.e.obj
	if s.g.replaced != NoObj {
		src = s.g.replaced // an as-enters flag branch's write belongs to the entering permanent (face_siegemode.go)
	}
	s.numStore(src, name, s.value(k, in.B), del, def)
	return true, true
}

// numClear drops o's numbers: o has just left the battlefield (CR 400.7; gorge
// clears Object.RuntimeSVars in the wasBattlefield arm of the zone move).
func (s *State) numClear(o Obj) {
	n := s.osDrop(func(r *osRec) bool { return r.src == o && r.osKind() == osNum })
	if DebugNumStoreStats != nil {
		DebugNumStoreStats.Cleared += n
	}
}
