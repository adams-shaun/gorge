package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Preparation (CR 722), cold: reached only when the table
// holds a FacePrepare card (genPrep.anyPrepare).
//
// A preparation permanent that becomes prepared (OpPrepare, gorge
// effAlterAttribute Prepared + grantPreparedCopy) mints a copy of its
// prepare spell in its controller's exile, an ordinary ofCopy object (the
// copy_gen.go precedent) appended at the end of that exile with no zone
// event and no trigger (gorge moves it inside Apply). The designation and
// the link to the copy ride one mayPlay record (0 B of State):
//
//	until = kop.DurPrepared, obj/ver = the exile copy,
//	turn  = source Obj | source ver<<8
//
// The permanent is prepared while some such record's source is on the
// battlefield with that ver (preparedSrc). The copy is cast from its
// controller's exile for its printed mana cost as gorge's "prepared_copy"
// (offerPreparedCasts); the cast's push removes the record (CR 722.3c,
// gorge pushCast's AlterAttribute -1), so the permanent is unprepared and
// the copy, once it leaves the stack, ceases. A copy whose source left the
// battlefield (or was unprepared) is unlinked and ceases at the next
// state-based check (ceaseTokensGen).

// DebugPrepareKeep keeps the DurPrepared record when the copy is cast
// (mutation testing only: the lockstep must catch it).
var DebugPrepareKeep bool

// DebugPrepareCease lets ceaseTokensGen cease a linked exile copy
// (mutation testing only).
var DebugPrepareCease bool

// DebugPrepareRegrant mints a new copy on a grant to an already prepared
// permanent (mutation testing only).
var DebugPrepareRegrant bool

// PrepareStats counts the lifted mechanic; oracle tests only.
type PrepareStats struct {
	Grants, NoOps, Casts, Ceased, SelfTrue, SelfFalse int
	Unprepares, TgtPrep, TgtNon                       int
	MaxMay, MaxObj                                    int
}

// DebugPrepareStats is the test hook; nil in normal play.
var DebugPrepareStats *PrepareStats

// preparedSrc reports whether DurPrepared record r still has its source: on
// the battlefield with the recorded ver (gorge clears Prepared as it leaves).
func (s *State) preparedSrc(r mayPlay) bool {
	src := &s.objs[uint8(r.turn)]
	return src.zone == kop.ZoneBattlefield && src.ver == uint8(r.turn>>8)
}

// prepared reports whether battlefield object o is prepared (gorge
// Object.Prepared): a live DurPrepared record names it as its source. The
// copy need not still be in exile.
func (s *State) prepared(o Obj) bool {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return false
	}
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.g.may[i]; r.until == kop.DurPrepared && Obj(uint8(r.turn)) == o && uint8(r.turn>>8) == ob.ver {
			return true
		}
	}
	return false
}

// preparedCopyLive reports whether exile copy o is still linked to a
// prepared source (gorge PreparedExileCopy): the record names it and its
// source is live. It is the exemption from ceasing and the offer's gate.
func (s *State) preparedCopyLive(k *prep, o Obj) (src Obj, ok bool) {
	if k.gen == nil || !k.gen.anyPrepare || DebugPrepareCease {
		return NoObj, false
	}
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.g.may[i]; r.until == kop.DurPrepared && r.obj == o && r.ver == ob.ver && s.preparedSrc(r) {
			return Obj(uint8(r.turn)), true
		}
	}
	return NoObj, false
}

// prepare is OpPrepare: object o (the replaced or the source) becomes
// prepared. Gorge's fold: only a battlefield preparation front that is not
// already prepared gains it, and the grant mints the exile copy.
func (s *State) prepare(k *prep, o Obj) {
	if o == NoObj {
		return
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return
	}
	cd := &k.t.Cards[s.cardAt(ob.slot)]
	if cd.FaceMode != kop.FacePrepare || cd.Face != 0 || len(cd.Faces) != 2 {
		return
	}
	if s.prepared(o) && !DebugPrepareRegrant {
		if st := DebugPrepareStats; st != nil {
			st.NoOps++
		}
		return
	}
	if s.objTableFull() {
		s.capFull(StallBoard)
		return
	}
	if s.g.nmay >= maxMay {
		s.capFull(StallMayPlay)
		return
	}
	ctrl := s.perms[ob.slot].ctrl
	pl := &s.pl[ctrl]
	if int(pl.exileN) >= len(pl.exile) {
		s.capFull(StallBoard)
		return
	}
	c := s.newToken(cd.Faces[1], ctrl)
	cp := &s.objs[c]
	cp.of = ofCopy
	cp.zone = kop.ZoneExile
	pl.exile[pl.exileN] = c
	pl.exileN++
	s.touch(o)
	s.g.may[s.g.nmay] = mayPlay{obj: c, ver: cp.ver, ctrl: ctrl, until: kop.DurPrepared, turn: uint16(o) | uint16(ob.ver)<<8}
	s.g.nmay++
	if st := DebugPrepareStats; st != nil {
		st.Grants++
		st.MaxMay = max(st.MaxMay, int(s.g.nmay))
		st.MaxObj = max(st.MaxObj, int(s.nobj))
	}
}

// execPrepare runs OpPrepare (execDefined's default branch); done=false
// means the op is not this one.
func (s *State) execPrepare(k *prep, in *kop.Instr) (ok, done bool) {
	if in.Op != kop.OpPrepare {
		return s.execMcombat(k, in) // filter_combatwords.go
	}
	o := s.refObjCtx(in.Obj, s.ctxCur())
	if st := DebugPrepareStats; st != nil && in.C == 0 && in.Obj != kop.RefSelf && in.Obj != kop.RefReplaced {
		s.prepTargetStat(k, o, st) // face_prepare.go
	}
	if in.C == 1 {
		s.unprepare(k, o) // face_prepare.go
		return true, true
	}
	s.prepare(k, o)
	return true, true
}

// selfPrepared is ExSelfPrepared: 1 if the source is prepared, else 0; N=1
// negates.
func (s *State) selfPrepared(x *kop.Expr, ctx evalCtx) int {
	v := 0
	if ctx.src != NoObj && s.prepared(ctx.src) {
		v = 1
	}
	if x.N == 1 {
		v ^= 1
	}
	if st := DebugPrepareStats; st != nil {
		if v == 1 {
			st.SelfTrue++
		} else {
			st.SelfFalse++
		}
	}
	return v
}

// offerPreparedCasts offers p's linked exile copies (gorge legal_walk_alt.go
// exileCastsWalk, "prepared_copy"): the source must be prepared on the
// battlefield under p's control; the copy is cast at its own face's timing
// for its printed cost (offered from the same mana census as the face casts:
// faceCastable).
func (s *State) offerPreparedCasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		ob := &s.objs[o]
		if ob.of&ofCopy == 0 {
			continue
		}
		src, ok := s.preparedCopyLive(k, o)
		if !ok || s.perms[s.objs[src].slot].ctrl != p {
			continue
		}
		if s.faceCastable(k, p, o, ob.card, sorc) {
			d.add(MakeAction(AkCast, uint8(o), 1, castFace))
		}
	}
}

// unprepareCast removes the DurPrepared record of spell copy o as its cast
// is pushed (CR 722.3c, 601.2a; gorge pushCast emits AlterAttribute -1
// before payment). A reversed cast restores nothing (CR 733.1 as gorge's
// abortCast: the permanent stays unprepared).
func (s *State) unprepareCast(o Obj) {
	ver := s.objs[o].ver
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.g.may[i]; r.until == kop.DurPrepared && r.obj == o && r.ver == ver {
			if DebugPrepareKeep {
				return
			}
			copy(s.g.may[i:s.g.nmay-1], s.g.may[i+1:s.g.nmay])
			s.g.nmay--
			if st := DebugPrepareStats; st != nil {
				st.Casts++
			}
			return
		}
	}
}

// ceaseQuiet ceases token or spell copy o where it sits in a graveyard (CR
// 704.5d-e, CR 111.8): nothing has left the zone, so no graveyard-leave trigger
// fires and Count$LeftGraveyardThisTurn doesn't count it (gorge events.IsCease).
func (s *State) ceaseQuiet(o Obj) {
	s.touch(o)
	ob := &s.objs[o]
	removeFrom(s.pl[ob.owner()].grave[:], &s.pl[ob.owner()].graveN, o)
	ob.zone = kop.ZoneNone
}

// Unprepare (OpPrepare C=1), cold. gorge's
// effAlterAttribute Activate$ False sends AlterAttribute Amount -1 to the
// referent: the fold clears Prepared and unlinks every copy whose
// PreparedSource is that permanent (apply_player.go), so the copy, no
// longer PreparedExileCopy, ceases at the next state-based check. It applies
// to any battlefield permanent, prepared or not (a no-op without a record).

// DebugPrepTargetNoUnprepare makes unprepare do nothing (mutation testing
// only: the lockstep must catch the copy staying castable).
var DebugPrepTargetNoUnprepare bool

// unprepare removes every DurPrepared record whose source is battlefield
// object o at its current ver (the prepared() scan, unprepareCast's
// compaction). A non-battlefield referent is untouched.
func (s *State) unprepare(k *prep, o Obj) {
	if o == NoObj || DebugPrepTargetNoUnprepare {
		return
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	for i := 0; i < int(s.g.nmay); {
		if r := s.g.may[i]; r.until == kop.DurPrepared && Obj(uint8(r.turn)) == o && uint8(r.turn>>8) == ob.ver {
			copy(s.g.may[i:s.g.nmay-1], s.g.may[i+1:s.g.nmay])
			s.g.nmay--
			if st := DebugPrepareStats; st != nil {
				st.Unprepares++
			}
			continue
		}
		i++
	}
}

// prepTargetStat counts a grant that names another permanent (a target slot
// or Defined$ Enchanted): on a preparation front (TgtPrep) or on anything
// else, which gorge skips and the kernel leaves untouched (TgtNon). Oracle
// tests only (DebugPrepareStats non-nil).
func (s *State) prepTargetStat(k *prep, o Obj, st *PrepareStats) {
	if o == NoObj {
		return
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return
	}
	if cd := &k.t.Cards[s.cardAt(ob.slot)]; cd.FaceMode == kop.FacePrepare && cd.Face == 0 {
		st.TgtPrep++
	} else {
		st.TgtNon++
	}
}
