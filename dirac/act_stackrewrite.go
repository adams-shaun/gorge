package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// two operations on stack entries that already
// exist. Cold and out of line (dispatched from execExt).
//
//   - ExtChangeTargets (CR 115.7; gorge effChangeTargets, choose_control.go):
//     "change the target of target spell or ability with a single target" /
//     Speedball's "you may choose new targets for that spell". The walk covers
//     the entry's root declaration and then each SubAbility$ link holding
//     announced targets, one ask per declaration; every answer is written
//     after the last ask, so the entry keeps its targets until then. Each
//     declaration's candidates are its slot's legal targets judged as the
//     entry itself (LegalRetargets: its controller, its source, itself as the
//     CR 115.5 self), so the resolving redirect may be named. Without Optional$
//     the targets must change (CR 115.7a): the current ones are not offered,
//     and unless every declaration has enough others, nothing is asked and
//     nothing moves. With Optional$ (CR 115.7d) the current ones are offered,
//     Done keeps the rest, and a pick that is already a target stays in place.
//     C bit 1 (DefinedMagnet$ Self with ChangeSingleTarget$ True, Spellskite)
//     asks nothing: ksrMagnet moves the first target of the first declaration
//     that offers the source to the source.
//   - ExtCounterAll (CR 701.5; gorge effCounter over Defined$ ValidStack
//     Spell.OppCtrl,Activated.OppCtrl,Triggered.OppCtrl, validStackTargets
//     L2576: the stack zone's arena order, bottom first): every entry an
//     opponent of the resolving controller controls is countered, an
//     uncounterable spell is skipped (gorge CounterAllowed), an ability is
//     removed with no card move. RememberCounteredSA$ counts one per
//     countered entry; the count joins the resolution's remembered count
//     (genState.remCnt, the channel ExRememberedCount sums), so the
//     Count$RememberedSize after it reads it. 0 B.

// DebugChangeTargetsOff makes ExtChangeTargets ask but keep the old target
// (the stackrewrite mutation test).
var DebugChangeTargetsOff bool

// raKsr is the ask of a redirect's new target.
const raKsr uint8 = 238

// ChKsrTarget is gorge's effChangeTargets KChoose ("Choose new target"):
// source = the object or stack ability chosen (a player: PlayerTarget0+p);
// AkDone keeps the old target (Optional$).
const ChKsrTarget ChoiceKind = 244

// ksrSubject is the stack index of the entry ExtChangeTargets rewrites, -1
// when there is none (it left the stack, or has no object to name).
func (s *State) ksrSubject(k *prep, in *kop.Instr) int {
	if slot, ok := tgtSlot(in.Obj); ok {
		c := s.tgtCode(slot)
		switch {
		case c == NoTarget:
			return -1
		case isStackTarget(c):
			return s.stackTgtIdx(c, s.res.e.tver[slot])
		case isObjTarget(c):
			ob := &s.objs[c]
			if ob.zone != kop.ZoneStack || ob.ver != s.res.e.tver[slot] {
				return -1
			}
			return s.stackIndexOf(Obj(c))
		}
		return -1
	}
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneStack {
		return -1
	}
	return s.stackIndexOf(o)
}

// ksrTotal is the number of targets e holds, extras of a multi-object slot
// included.
func (s *State) ksrTotal(e *stackEntry) int {
	var codes [kop.MaxTargets + maxXTgt]uint8
	return s.chainTargets(e, &codes)
}

// ksrFirstDecl is the first declaration slot of e in gorge's walk, -1 when
// there is none: a charm's head is the Charm node, whose own census names
// nothing.
func ksrFirstDecl(k *prep, e *stackEntry) int {
	if e.ab == 255 {
		if tspecE(k, e, 0) == nil {
			return -1
		}
		return 0
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.NModes != 0 {
		return -1
	}
	for i := 0; i < int(a.NTargets); i++ {
		if ksrIsDecl(a, i) {
			return i
		}
	}
	return -1
}

// ksrIsDecl reports whether slot i of a is declared at announcement: a spell's
// root slot (TgfDecl) or a chain link's (TgfDeclSub), or an ability's own slot.
func ksrIsDecl(a *kop.Ability, i int) bool {
	t := &a.Targets[i]
	if t.TFlags&kop.TgfAtResolution != 0 {
		return false
	}
	return a.Kind != kop.AbSpell || t.TFlags&(kop.TgfDecl|kop.TgfDeclSub) != 0
}

// ksrNextDecl is the declaration slot after slot after, -1 when none is left.
func ksrNextDecl(k *prep, e *stackEntry, after int) int {
	if e.ab == 255 {
		return -1
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	for i := after + 1; i < int(a.NTargets); i++ {
		if ksrIsDecl(a, i) {
			return i
		}
	}
	return -1
}

// ksrPicksFor counts the answers already given for declaration slot d.
func (s *State) ksrPicksFor(d int) int {
	n := 0
	for i := 0; i < int(s.res.peekN); i++ {
		if int(s.res.peek[i]) == d {
			n++
		}
	}
	return n
}

// ksrOffered writes declaration slot d's candidates for the entry at stack
// index idx. The census reads the entry as the one being targeted for: a
// filter's entry is the subject, whose parent slot holds the answer already
// given for it. must leaves its current targets out.
func (s *State) ksrOffered(k *prep, idx, d int, must bool, out *[MaxOptions]uint8) int {
	e := &s.stack[idx]
	t := tspecE(k, e, d)
	if t == nil {
		return 0
	}
	src := NoSlot
	if e.kind != seSpell {
		ob := &s.objs[e.obj]
		if ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
			src = ob.slot
		}
	}
	saved := s.res.e
	s.res.e = *e
	for i := int(s.res.peekN) - 1; i >= 0; i-- {
		if sl := int(s.res.peek[i]); sl < kop.MaxTargets && s.ksrPicksFor(sl) == 1 {
			s.res.e.tgt[sl] = uint8(s.res.arr[i])
		}
	}
	n := s.targetCands(k, t, e.ctrl, src, e.obj, out)
	s.res.e = saved
	if e.kind != seSpell {
		// an ability is no legal target for itself (CR 115.5): the entry's own
		// stack code (a spell's own object is the self targetCands skips)
		self, m := StackTarget0+uint8(idx), 0
		for _, c := range out[:n] {
			if c != self {
				out[m] = c
				m++
			}
		}
		n = m
	}
	if !must {
		return n
	}
	var old [MaxOptions]uint8
	nold := s.copySlotTargets(e, d, &old)
	m := 0
next:
	for _, c := range out[:n] {
		for _, o := range old[:nold] {
			if o == c {
				continue next
			}
		}
		out[m] = c
		m++
	}
	return m
}

// changeTargets is ExtChangeTargets; false: the ask is posed.
//
//go:noinline
func (s *State) changeTargets(k *prep, in *kop.Instr) bool {
	idx := s.ksrSubject(k, in)
	if idx < 0 {
		return true
	}
	e := &s.stack[idx]
	total := s.ksrTotal(e)
	if total == 0 {
		return true
	}
	must := in.C&1 == 0
	magnet := in.C&2 != 0
	s.res.peekN = 0
	var old, buf [MaxOptions]uint8
	covered := 0
	for d := ksrFirstDecl(k, e); d >= 0; d = ksrNextDecl(k, e, d) {
		nold := s.copySlotTargets(e, d, &old)
		covered += nold
		if must && !magnet && nold > 0 && s.ksrOffered(k, idx, d, true, &buf) < nold {
			return true
		}
	}
	if d := ksrFirstDecl(k, e); d < 0 {
		return true // a charm's head: its census names nothing, so gorge moves nothing
	}
	if covered != total {
		s.capFull(StallMultiRetarget) // a target no declaration holds
		return true
	}
	s.res.left = uint8(idx) + 1
	if magnet {
		return s.ksrMagnet(k, e, must)
	}
	return s.ksrAskFrom(k, ksrFirstDecl(k, e), must)
}

// ksrMagnet is the DefinedMagnet$ Self with ChangeSingleTarget$ True walk
// (gorge effChangeTargets' magnet and single arms): no ask. The first
// declaration, in the walk's order, whose offer holds the resolving ability's
// source (a creature or player-less object on the battlefield, as the source
// still is) takes it in place of its first target, and the walk stops there;
// a declaration whose offer lacks it is passed over (a source that is already a
// target is not offered when the targets must change).
func (s *State) ksrMagnet(k *prep, e *stackEntry, must bool) bool {
	r := &s.res
	idx := int(r.left) - 1
	src := r.e.obj
	var old, buf [MaxOptions]uint8
	if ob := &s.objs[src]; ob.zone == kop.ZoneBattlefield && ob.ver == r.e.srcVer {
		for d := ksrFirstDecl(k, e); d >= 0; d = ksrNextDecl(k, e, d) {
			if s.copySlotTargets(e, d, &old) == 0 {
				continue
			}
			if n := s.ksrOffered(k, idx, d, must, &buf); ksrHas(buf[:n], uint8(src)) {
				r.peek[0], r.arr[0], r.peekN = Obj(d), src, 1
				break
			}
		}
	}
	s.ksrApply(k, &s.stack[idx])
	r.left, r.peekN = 0, 0
	return true
}

// ksrAskFrom poses the first declaration at or after slot d with something to
// answer. When none is left it writes the answers; true: nothing is posed.
func (s *State) ksrAskFrom(k *prep, d int, must bool) bool {
	idx := int(s.res.left) - 1
	e := &s.stack[idx]
	var old, buf [MaxOptions]uint8
	for ; d >= 0; d = ksrNextDecl(k, e, d) {
		if s.copySlotTargets(e, d, &old) > 0 && s.ksrOffered(k, idx, d, must, &buf) > 0 {
			s.res.chosen = uint8(d)
			s.res.ask, s.res.askP = raKsr, s.res.e.ctrl
			return false
		}
	}
	s.ksrApply(k, e)
	s.res.left, s.res.peekN = 0, 0
	return true
}

// ksrApply writes the answers kept in res.arr into e, declaration by
// declaration, and reports each target that became one: every target of a root
// declaration an answer was given for (gorge recordTargets), and only the
// arrivals on a chain link (RetargetLink).
func (s *State) ksrApply(k *prep, e *stackEntry) {
	if DebugChangeTargetsOff {
		return
	}
	for d := ksrFirstDecl(k, e); d >= 0; d = ksrNextDecl(k, e, d) {
		var old, out [MaxOptions]uint8
		var picks [8]uint8
		np := 0
		for i := 0; i < int(s.res.peekN); i++ {
			if int(s.res.peek[i]) == d {
				picks[np] = uint8(s.res.arr[i])
				np++
			}
		}
		if np == 0 {
			continue
		}
		nold := s.copySlotTargets(e, d, &old)
		ksrRetargeted(&old, nold, &picks, np, &out)
		s.ksrWrite(e, d, &out)
		for i := 0; i < nold; i++ {
			if tspecE(k, e, d).TFlags&kop.TgfDeclSub == 0 || !ksrHas(old[:nold], out[i]) {
				s.targetedOne(k, e, out[i])
			}
		}
	}
}

// ksrHas reports whether c is among codes.
func ksrHas(codes []uint8, c uint8) bool {
	for _, o := range codes {
		if o == c {
			return true
		}
	}
	return false
}

// ksrRetargeted is gorge's retargeted: a pick that is already a target stays
// where it is, the others take the remaining places in order, and a place no
// pick reaches keeps its target (CR 115.7e).
func ksrRetargeted(old *[MaxOptions]uint8, nold int, picks *[8]uint8, np int, out *[MaxOptions]uint8) {
	var moved [8]uint8
	nm := 0
	for _, c := range picks[:np] {
		if !ksrHas(old[:nold], c) {
			moved[nm] = c
			nm++
		}
	}
	m := 0
	for i := 0; i < nold; i++ {
		out[i] = old[i]
		if m < nm && !ksrHas(picks[:np], old[i]) {
			out[i] = moved[m]
			m++
		}
	}
}

// ksrWrite writes out into slot d's places in the order copySlotTargets reads
// them.
func (s *State) ksrWrite(e *stackEntry, d int, out *[MaxOptions]uint8) {
	n := 0
	if d < int(e.nt) && e.tgt[d] != NoTarget {
		c := out[0]
		e.tgt[d] = c
		e.tver[d] = s.ksrVer(c)
		n = 1
	}
	if e.xi != 0 {
		x := s.xtOf(e)
		for j := 0; j < int(x.n); j++ {
			if x.slot[j] == uint8(d) && n < MaxOptions {
				x.code[j] = out[n]
				x.ver[j] = s.ksrVer(out[n])
				n++
			}
		}
	}
}

// ksrVer is the version a target code records.
func (s *State) ksrVer(c uint8) uint8 {
	switch {
	case isObjTarget(c):
		return s.objs[c].ver
	case isStackTarget(c):
		return s.stackTgtSid(c)
	}
	return 0
}

// askOptionsStackRewrite lists the new-target pick (the tail of askOptionsBreach).
//
//go:noinline
func (s *State) askOptionsStackRewrite(k *prep, d *Decision) bool {
	if s.res.ask != raKsr {
		return false
	}
	optional := s.prog(k)[s.res.pc].C&1 != 0
	var buf [MaxOptions]uint8
	n := s.ksrOffered(k, int(s.res.left)-1, int(s.res.chosen), !optional, &buf)
	d.Sub = ChKsrTarget
next:
	for i := 0; i < n; i++ {
		for j := 0; j < int(s.res.peekN); j++ {
			if s.res.peek[j] == Obj(s.res.chosen) && uint8(s.res.arr[j]) == buf[i] {
				continue next
			}
		}
		d.add(MakeAction(AkChoose, buf[i], 0, 0))
	}
	if optional {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}

// applyAskStackRewrite records a pick (the tail of applyAskBreach). A declaration's ask
// closes on Done or when its places are full, and the entry takes every answer
// after the last declaration's.
//
//go:noinline
func (s *State) applyAskStackRewrite(k *prep, a Action) bool {
	if s.res.ask != raKsr {
		return false
	}
	r := &s.res
	idx := int(r.left) - 1
	d := int(r.chosen)
	must := s.prog(k)[r.pc].C&1 == 0
	if a.Kind() == AkChoose {
		if int(r.peekN) >= len(r.peek) {
			s.capFull(StallMultiRetarget)
			return true
		}
		r.peek[r.peekN], r.arr[r.peekN] = Obj(d), Obj(a.Source())
		r.peekN++
		var old, buf [MaxOptions]uint8
		limit := min(s.copySlotTargets(&s.stack[idx], d, &old), s.ksrOffered(k, idx, d, must, &buf))
		if s.ksrPicksFor(d) < limit {
			return true // the next pick of this declaration (Step poses it)
		}
	}
	r.ask = raNone
	if !s.ksrAskFrom(k, ksrNextDecl(k, &s.stack[idx], d), must) {
		return true
	}
	s.continueRes(k, 0)
	return true
}

// counterAll is ExtCounterAll.
//
//go:noinline
func (s *State) counterAll(k *prep) {
	me := s.res.e.ctrl
	for i := 0; i < int(s.sp); {
		e := &s.stack[i]
		if e.ctrl == me || e.kind == seSpell && s.uncounterable(k, i) {
			i++
			continue
		}
		s.g.remCnt++
		s.counterAt(k, i)
	}
}

// ksrSingle is gorge's singleTarget property: the entry's chain holds exactly
// one target, Targets plus SubTargets, so a target named twice counts twice.
func (s *State) ksrSingle(e *stackEntry) bool {
	return (s.ksrTotal(e) == 1) != DebugTrigParams4Flip
}

// ksrSingleSpell is TgfSingleTgt for the spell o.
func (s *State) ksrSingleSpell(o Obj) bool {
	i := s.stackIndexOf(o)
	return i >= 0 && s.ksrSingle(&s.stack[i])
}
