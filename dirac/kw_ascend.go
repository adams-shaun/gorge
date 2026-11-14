package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ascend and the city's blessing (CR 702.131), copied from gorge
// (engine/rules/ascend.go checkBlessingGrants and grantSpellBlessing; the
// readers effects/conditions.go's Blessing condition, layers_static.go's
// condBlessing and count_body_heads.go's Count$Blessing.<yes>.<no>). Cold and
// out of line: a table without a printed Ascend card (CardDef.Ascend) never
// takes these paths (prep.ascend).
//
// State: the blessing is one bit per seat that never clears, bit
// blessShift+p of genState.cast1 (its low nibble is the first-cast bits,
// cast_firstcast.go, never cleared either), so State does not grow.
//
//   - 702.131b, the permanent form: a post-placement scan (blessScan) from
//     the one battlefield entry (enterNF: spells, tokens, copies, returns),
//     from control changes (ctrlMove) and from the state-based pass (what
//     neither hook sees: a phase-in, a carrier regaining its abilities). An
//     unblessed seat still in the game that controls ten or more phased-in
//     permanents, one of them carrying Ascend and not having lost its
//     abilities, gets the blessing.
//   - 702.131a, the spell form: an instant or sorcery carrying Ascend grants
//     its controller the blessing as it resolves, after the fizzle check and
//     before its program reads the latch, when they control ten or more
//     permanents (gorge counts the controller's whole battlefield list, a
//     phased-out permanent included).

// DebugBlessingOff (TestAscendBlessingLowers' mutation): neither form ever
// grants the blessing.
var DebugBlessingOff bool

// BlessStats counts the mechanic's events; oracle tests only.
type BlessStats struct {
	Permanent int // the permanent form granted a seat the blessing
	Spell     int // a resolving instant or sorcery granted it
	ExYes     int // ExBlessing read its Sub[0] arm
	ExNo      int // ExBlessing read its Sub[1] arm
	CondYes   int // CondBlessing held
	CondNo    int // CondBlessing did not hold
}

// DebugBlessStats is the test hook; nil in normal play.
var DebugBlessStats *BlessStats

// blessShift is the first blessing bit of genState.cast1: seats take bits
// blessShift..blessShift+MaxSeats-1, above the first-cast bits.
const blessShift = 4

// the blessing bits fit the byte at every seat class
const _ = uint8(1<<(blessShift+MaxSeats) - 1)

// blessTen is CR 702.131's "ten or more permanents".
const blessTen = 10

func blessMask(p uint8) uint8 {
	if p >= MaxSeats {
		return 0
	}
	return 1 << (blessShift + p)
}

// blessed reports whether seat p has the city's blessing; an out-of-range
// seat has none (gorge's readers deny it).
func (s *State) blessed(p uint8) bool { return s.g.cast1&blessMask(p) != 0 }

// Blessed reports whether seat p has the city's blessing (oracle / tests).
func (s *State) Blessed(p uint8) bool { return s.blessed(p) }

// tableHasAscend reports whether some card of t prints Ascend.
func tableHasAscend(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Ascend {
			return true
		}
	}
	return false
}

// blessBoard is seat p's battlefield as gorge's per-controller list holds it:
// the phased-in permanents it controls (in), and those plus its phased-out
// ones (all; a phased-out permanent is off every mask, kw_phasing.go).
func (s *State) blessBoard(k *prep, p uint8) (in, all slotMask) {
	in = s.ctrlM[p]
	all = in
	if k.phasing {
		for m := s.phasedSlots(); m.any(); m = m.rest() {
			if sl := m.first(); s.perms[sl].ctrl == p {
				all = all.with(sl)
			}
		}
	}
	return in, all
}

// blessScan is gorge checkBlessingGrants: each unblessed seat still in the
// game that controls ten or more phased-in permanents, one of them (gorge
// walks the whole list) carrying Ascend, gets the blessing.
//
//go:noinline
func (s *State) blessScan(k *prep) {
	if s.over || DebugBlessingOff {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.blessed(p) || s.pl[SeatIx(p)].lost {
			continue
		}
		in, all := s.blessBoard(k, p)
		if in.count() < blessTen {
			continue
		}
		for m := all; m.any(); m = m.rest() {
			sl := m.first()
			if k.t.Cards[s.cardAt(sl)].Ascend && !s.genLost(k, sl) {
				s.g.cast1 |= blessMask(p)
				if st := DebugBlessStats; st != nil {
					st.Permanent++
				}
				break
			}
		}
	}
}

// spellBlessing is gorge grantSpellBlessing: the resolving instant or sorcery
// e carries Ascend, so its controller, still in the game, gets the blessing
// when they control ten or more permanents.
//
//go:noinline
func (s *State) spellBlessing(k *prep, e *stackEntry) {
	p := e.ctrl
	if DebugBlessingOff || !k.t.Cards[e.card].Ascend || p >= s.nSeats() || s.blessed(p) || s.pl[SeatIx(p)].lost {
		return
	}
	if _, all := s.blessBoard(k, p); all.count() < blessTen {
		return
	}
	s.g.cast1 |= blessMask(p)
	if st := DebugBlessStats; st != nil {
		st.Spell++
	}
}

// exBlessing evaluates ExBlessing: Sub[0] when the reading controller has the
// city's blessing, else Sub[1].
func (s *State) exBlessing(k *prep, x *kop.Expr, ctx evalCtx) int {
	st := DebugBlessStats
	if s.blessed(ctx.ctrl) {
		if st != nil {
			st.ExYes++
		}
		return s.evalV(k, x.Sub[0], ctx)
	}
	if st != nil {
		st.ExNo++
	}
	return s.evalV(k, x.Sub[1], ctx)
}

// condBlessing evaluates CondBlessing for the controller ctrl.
func (s *State) condBlessing(ctrl uint8) bool {
	r := s.blessed(ctrl)
	if st := DebugBlessStats; st != nil {
		if r {
			st.CondYes++
		} else {
			st.CondNo++
		}
	}
	return r
}
