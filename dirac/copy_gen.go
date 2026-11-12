package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Spell copies (OpCopySpell: Thousand-Year Storm), mirroring gorge
// effects/copy.go effCopySpellAbility and the StackCopy fold
// (events/apply_copy.go): a copy is a new card object on the stack (gorge
// AddObject, IsCopy) controlled by the copier, with the copied spell's
// targets, modes, X and kicked status. It is not cast (no cast triggers, no
// cast count). It resolves like the spell; leaving the stack it rests in
// exile and ceases to exist there (CR 707.10a: gorge's spellRestZone, then
// the state-based check in ceaseTokensGen).
//
// A copy made with "you may choose new targets" (MayChooseTarget$) elects them
// inside the resolution that made it, before anyone receives priority (CR
// 707.10c): copyElect poses gorge's AskCopyTargets (rules/target_modes.go) once
// per target declaration -- the spell's root node, or the chosen mode's head
// (the slots kgen marks TgfDecl): a KTarget whose options are every inherited
// target first (kept even when illegal), then the declaration's other legal
// targets in the cast census order. The answer replaces the copy's whole target
// list (recordChosenTargets without append): the declaration's slot gets it, the
// other cast-time slots are cleared.
//
// A declaration that takes several objects (Run Away Together, Macabre
// Waltz) is asked as sequential picks closed by Done (raGenCopyTargets). A
// divided declaration (Fire) is asked the same way, with the card's own
// bounds; the copy inherits the cast-time division, and gorge's
// AnnouncedDivision only applies it while the target count is unchanged.

// SubCopyTarget is a DkTarget decision's Sub for a spell copy's new-target
// election: its options are in gorge's order (the inherited targets first),
// not the cast census order.
const SubCopyTarget ChoiceKind = 1

// isSpellCopy reports whether o is a copy of a spell (not a token).
func (s *State) isSpellCopy(o Obj) bool {
	return s.objs[o].of&(ofCopy|ofToken) == ofCopy
}

// FeatStackCopy reports whether stack object o is a copy of a spell put there
// without being cast. A copy that is cast (cast_copycast.go) is a free cast.
func (s *State) FeatStackCopy(o Obj) bool {
	if !s.isSpellCopy(o) {
		return false
	}
	e := s.spellEntryOf(o)
	return e != nil && e.flags&sfFree == 0
}

// execCopySpell runs OpCopySpell (stalling the game at capacity); false: a
// copy's election is posed.
func (s *State) execCopySpell(k *prep, in *kop.Instr) bool {
	if s.res.e.ab == abStorm {
		return s.copySpells(k, in, int(s.res.e.trigAmt)) // the count fixed when the cast trigger fired (kw_storm.go)
	}
	return s.copySpells(k, in, s.value(k, in.A))
}

// copySpells makes OpCopySpell's n copies.
func (s *State) copySpells(k *prep, in *kop.Instr, n int) bool {
	if in.Obj == kop.RefSelf {
		return s.copyParentSpell(k, in, n) // kw_madness.go
	}
	return s.copySpell(k, s.objRefObj(in.Obj), n, in.C&1 != 0)
}

// copySpell copies the spell src (on the stack below the resolving entry) n
// times for the resolving entry's controller; mayRetarget lets the copy's
// controller choose new targets (CR 707.10). False: a copy's election is posed,
// and copyResume makes the rest.
func (s *State) copySpell(k *prep, src Obj, n int, mayRetarget bool) bool {
	e := &s.res.e
	if src == NoObj || s.objs[src].zone != kop.ZoneStack {
		return true
	}
	si := -1
	for i := int(s.sp) - 2; i >= 0; i-- { // below the resolving entry
		if s.stack[i].kind == seSpell && s.stack[i].obj == src {
			si = i
			break
		}
	}
	if si < 0 {
		return true
	}
	for c := n - 1; c >= 0; c-- {
		if !s.mintSpellCopy(k, si, e.ctrl) {
			return true
		}
		if mayRetarget && s.copyElect(k, uint8(c)) {
			return false
		}
	}
	return true
}

// mintSpellCopy puts one copy of the spell at stack index si on top of the
// stack, controlled by ctrl (gorge's StackCopy fold: a new card object with
// the copied spell's targets, modes, X and kicked status). It stalls and
// returns false at capacity.
func (s *State) mintSpellCopy(k *prep, si int, ctrl uint8) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return false
	}
	if int(s.sp) >= MaxStack {
		s.stall(StallStack)
		return false
	}
	ce := s.stack[si]
	if k.gen.anyPermCopy && s.permCopyBlocked(k, &ce) {
		return false
	}
	o := s.newToken(ce.card, ctrl)
	s.objs[o].of = ofCopy
	s.objs[o].zone = kop.ZoneStack
	ce.obj, ce.ctrl, ce.srcVer = o, ctrl, s.objs[o].ver
	ce.flags &= sfKicked | sfPromised // the copy keeps the gift election (CR 707.10)
	if cd := &k.t.Cards[ce.card]; cd.Bargain || cd.Teamwork != 0 {
		// a copy is never cast, so it carries no FlagBargained or
		// FlagTeamworkPaid (CR 707.10; kw_bargain.go, kw_teamwork.go)
		ce.flags &^= sfKicked
	}
	// the copy was not warped and is not a warp recast (CR 702.185a):
	// the cast's provenance does not copy, so no delayed exile is armed
	ce.mode &^= modeWarped | modeWarpRecast
	if ce.trigPl == trigPlWarpG {
		ce.trigPl = 0 // nor warped through a grant (kw_warpgrant.go)
	}
	if ce.trigPl == trigPlAirbend {
		ce.trigPl = 0 // nor airbent (act_airbend.go)
	}
	if ce.impended() {
		ce.mode &^= modeImpended // nor impended (CR 707.10: gorge's FlagImpending is cast provenance)
	}
	ce.remVer &= spellFirstCast          // nor the keywords the spell gained (CR 707.2); the first-cast stamp stays (cast_firstcast.go)
	ce.prov, ce.spc, ce.etbSrc = 0, 0, 0 // nor cast from a zone, nor convoked, nor spent colours, nor the rider's source
	if ce.kind == seSpell {
		ce.trigAmt = 0 // nor any mana spent to cast it (CR 707.10: a copy is not cast; "mana spent to cast this spell" reads 0)
	}
	ce.sid = 0 // nor its FINALITY riders: a spell's sid is the rider count (trig_mayplaystatic.go)
	// nor was it sneaked (CR 707.10: gorge's FlagSneaked is a cast
	// provenance flag): it enters as an ordinary permanent
	if ce.trigPl == trigPlMayhem {
		ce.trigPl = 0 // nor mayhem-cast (kw_mayhem.go)
	}
	if ce.trigPl == trigPlSneak {
		ce.trigPl, ce.trigObj = 0, NoObj
	}
	if ce.trigPl == trigPlSuspend {
		ce.trigPl = 0 // nor cast by suspend (CR 707.10: gorge's FlagSuspend is cast provenance; kw_suspend.go)
	}
	if ce.trigPl == trigPlWebSling {
		ce.trigPl = 0 // nor web-slinged (CR 707.10: gorge's FlagWebSlinged; kw_websling.go)
	}
	if ce.xi != 0 {
		from := s.g.xtp[ce.xi-1]
		ce.xi = 0
		xt := s.xtMut(&ce)
		if xt == nil {
			return false
		}
		*xt = from
	}
	// below the resolving trigger, which leaves the top when it finishes
	top := s.sp - 1
	s.stack[top+1] = s.stack[top]
	s.stack[top] = ce
	s.sp++
	if k.gen.anyOrCopy {
		s.fireCopyTrigs(k, &s.stack[top])
	}
	return true
}

// copyDeclSlot is the target slot of entry e's copy declaration, or -1
// when gorge would not ask (no declaration).
func (s *State) copyDeclSlot(k *prep, e *stackEntry) int {
	if e.kind != seSpell {
		return s.copyDeclSlotAb(k, e) // an ability copy's declaration (copy_ability.go)
	}
	if e.ab == 255 {
		return s.auraCopyDeclSlot(k, e) // a permanent spell has no spell ability to declare targets, but an Aura spell's host (copy_aura.go)
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.TFlags&kop.TgfDecl == 0 {
			continue
		}
		if tm := a.TgtModes[i]; tm != 0 && tm&a.ModeSet(e.mode) == 0 {
			continue // a mode the copied spell did not choose
		}
		return i
	}
	return -1
}

// copyBounds is the copy ask's [min, max] for slot (gorge
// resolvedTargetBounds, then oneEachTargetBounds for a different-controllers
// slot, then the clamps to the n offered), and whether it is a multi-pick.
func (s *State) copyBounds(k *prep, e *stackEntry, slot int, opts *Decision) (lo, hi uint8, multi bool) {
	t := tspecE(k, e, slot)
	lo, hi = 1, 1
	if t.Min|t.Max != 0 {
		lo, hi = t.Min, t.Max
	}
	if t.UpTo {
		lo = 0
	}
	if t.TFlags&kop.TgfKickedRange != 0 && e.flags&sfKicked != 0 {
		lo, hi = t.MinKicked, t.MaxKicked
	} else if slotNoneUnkicked(t) {
		lo, hi = 0, 0
	}
	if t.TFlags&kop.TgfGiftRange != 0 {
		lo, hi = giftRange(t, e.promised())
	}
	if t.TFlags&kop.TgfBoundsV != 0 {
		// gorge re-resolves the bounds for the copy (target_modes.go
		// copyTargets: resolvedTargetBounds(controller, copy, sa, copy.X))
		l, h := s.tgtBoundsEval(k, t, e)
		lo, hi = uint8(min(l, 254)), uint8(min(h, 254))
	}
	multi = hi > 1 || e.xi != 0 && s.slotHasExtra(e, uint8(slot))
	if t.TFlags&kop.TgfDiffController != 0 {
		var g [2]bool
		for i := 0; i < int(opts.N); i++ {
			g[s.copyGroup(opts.Options[i].Target())] = true
		}
		if n := b2u(g[0]) + b2u(g[1]); n > 0 && hi > n {
			hi = n
		}
	}
	if uint16(lo) > opts.N {
		lo = uint8(opts.N)
	}
	if hi < lo {
		hi = lo
	}
	return lo, hi, multi
}

func b2u(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// copyGroup is target code c's controller seat (gorge
// candidateControllerSeat: a player its own seat, an object its controller).
func (s *State) copyGroup(c uint8) uint8 {
	if isPlayerOrNone(c) {
		return SeatIx(CodeSeat(c))
	}
	return SeatIx(s.ctrlOfObj(Obj(c)))
}

// copyHold is the resolution that made a copy, waiting while the copy's
// controller elects its targets: how many copies it has made, and how many more
// it owes after this one.
type copyHold struct {
	res        resCtx
	made, left uint8
}

// copyElect poses the new-target election of the copy just minted under the
// resolving entry, which waits below it (gorge effects emitCopy -> AskCopyTargets); false = nothing to
// ask. A declaration of several objects (Run Away Together, Macabre Waltz) is
// gorge's one subset decision posed as sequential picks closed by Done
// (res.arr holds the picks, res.peekN their count, res.chosen/peekI the
// bounds); the answer replaces the declaration's targets when it closes. left
// is what copyResume makes once the election closes.
func (s *State) copyElect(k *prep, left uint8) bool {
	h := &s.cold.copyHeld[0]
	h.res, h.left = s.res, left
	h.made++
	s.copierUnder(h)
	w := s.stack[s.sp-1] // the working copy the answers land in until the last declaration closes (copy_subchain.go)
	if s.copyOwnTargets(k, &w) && s.copyAskFrom(k, &w, s.copyNextDecl(k, &w, -1)) {
		return true
	}
	s.copyRelease(h)
	return false
}

// copierUnder moves the resolving entry under the copies it has made for the
// election: gorge's copy goes on top of the entry that made it, and a target the
// copy names is a stack position. Everywhere else the resolving entry stays on
// top, because finishResolve pops it.
func (s *State) copierUnder(h *copyHold) {
	top, n := int(s.sp)-1, int(h.made)
	cop := s.stack[top]
	copy(s.stack[top-n+1:top+1], s.stack[top-n:top])
	s.stack[top-n] = cop
}

// copyRelease puts the resolving entry back on top and gives the resolution back,
// its count of copies kept while it owes more.
func (s *State) copyRelease(h *copyHold) {
	top, n := int(s.sp)-1, int(h.made)
	cop := s.stack[top-n]
	copy(s.stack[top-n:top], s.stack[top-n+1:top+1])
	s.stack[top] = cop
	s.res = h.res
	if h.left == 0 {
		h.made = 0
	}
	h.res, h.left = resCtx{}, 0
}

// copyResume goes back to the resolution that made the copy once its election
// has closed.
func (s *State) copyResume(k *prep) {
	h := &s.cold.copyHeld[0]
	left := h.left
	s.copyRelease(h)
	if left != 0 {
		in := &s.prog(k)[s.res.pc]
		var more bool
		if in.Op == kop.OpDemonstrate {
			more = s.demonstrateCopy(k, left)
		} else if in.Ext() == kop.ExtCopyAbility {
			more = s.copyAbilityN(k, in, int(left))
		} else {
			more = s.copySpells(k, in, int(left))
		}
		if !more {
			return
		}
	}
	s.continueRes(k, 0)
}

// copyInherited lists the copy's inherited targets in gorge's order: each
// cast-time slot's target, the declaration slot's other objects after its
// first.
func (s *State) copyInherited(e *stackEntry, slot int, out *[MaxOptions]uint8) int {
	n := 0
	k := s.k()
	own := s.copyOwnSlot(k, e, slot) // a chain link's or a mode's election lists its own slot alone
	for i := 0; i < int(e.nt); i++ {
		if e.tgt[i] != NoTarget && (own && i == slot || !own && !s.copySubSlot(k, e, i)) {
			out[n] = e.tgt[i]
			n++
		}
		if i == slot && e.xi != 0 {
			x := s.xtOf(e)
			for j := 0; j < int(x.n); j++ {
				if x.slot[j] == uint8(slot) && n < MaxOptions {
					out[n] = x.code[j]
					n++
				}
			}
		}
	}
	return n
}

// copyTargetOptions: the inherited targets in slot order, then the
// declaration's other legal targets (targetCands' census order). For a
// multi-pick, the picks so far are left out, and so is an option no
// completion to the minimum can follow (the oracle's gorge side offers only
// completable picks); Done once the minimum is met.
func (s *State) copyTargetOptions(k *prep, d *Decision, slot int) {
	e := &s.res.e
	d.Kind, d.Sub = DkTarget, SubCopyTarget
	d.Player = e.ctrl
	var all [MaxOptions]uint8
	n := s.copyInherited(e, slot, &all)
	t := tspecE(k, e, slot)
	var buf [MaxOptions]uint8
	nc := s.targetCands(k, t, e.ctrl, s.resSrcSlot(k), e.obj, &buf)
next:
	for _, c := range buf[:nc] {
		for j := 0; j < n; j++ {
			if all[j] == c {
				continue next
			}
		}
		if n < MaxOptions {
			all[n] = c
			n++
		}
	}
	n = s.copyUniqueLink(k, e, slot, &all, n)
	if s.res.ask != raGenCopyTargets {
		for _, c := range all[:n] {
			d.add(MakeAction(AkTarget, 0, c, 0))
		}
		return
	}
	diff := t.TFlags&kop.TgfDiffController != 0
	picked := s.res.arr[:s.res.peekN]
	ok := func(c uint8, extra uint8, withExtra bool) bool {
		for _, p := range picked {
			if Obj(c) == p || diff && s.copyGroup(c) == s.copyGroup(uint8(p)) {
				return false
			}
		}
		if withExtra && (c == extra || diff && s.copyGroup(c) == s.copyGroup(extra)) {
			return false
		}
		return true
	}
	need := int(s.res.chosen) - int(s.res.peekN) - 1 // picks still owed after this one
	for _, c := range all[:n] {
		if !ok(c, 0, false) {
			continue
		}
		if need > 0 {
			m := 0
			for _, c2 := range all[:n] {
				if c2 != c && ok(c2, c, true) {
					m++
				}
			}
			if m < need {
				continue
			}
		}
		d.add(MakeAction(AkTarget, 0, c, 0))
	}
	if s.res.peekN >= s.res.chosen && d.N > 0 || d.N == 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
}

// applyCopyTarget records the answer and resolves the copy.
func (s *State) applyCopyTarget(k *prep, a Action) {
	i := int(s.res.left)
	var picks []Obj
	if s.res.ask == raGenCopyTargets {
		if a.Kind() == AkTarget {
			s.res.arr[s.res.peekN] = Obj(a.Target())
			s.res.peekN++
			if s.res.peekN < s.res.peekI && s.res.peekN < uint8(len(s.res.arr)) {
				return // the next pick (Step poses it)
			}
		}
		picks = s.res.arr[:s.res.peekN]
		s.setCopyTargetsQ(k, &s.res.e, i, picks)
	} else {
		picks = []Obj{Obj(a.Target())}
		s.setCopyTargetsQ(k, &s.res.e, i, picks)
	}
	s.res.ask = raNone
	if s.copyAnswered(k, i, picks) {
		return // the next declaration's election (Step poses it)
	}
	s.copyResume(k)
}

// setCopyTargetsQ replaces the copy's targets with the answer (gorge
// recordChosenTargets without append): slot i gets the picks (the first in
// tgt[i], the others in the entry's extra targets), the other cast-time
// slots are cleared.
func (s *State) setCopyTargetsQ(k *prep, e *stackEntry, i int, picks []Obj) {
	if e.ab == 255 {
		s.setAuraCopyTarget(e, picks) // copy_aura.go
		return
	}
	a0 := &k.t.Cards[e.card].Abilities[e.ab]
	oldN := 0
	if a0.Targets[i].Divide != 0 {
		var inherited [maxXTgt + 1]uint8
		oldN = s.divPicks(e, uint8(i), &inherited)
	}
	for j := 0; j < int(e.nt); j++ {
		if j != i && !copyKeepSlot(a0, i, j) {
			e.tgt[j] = NoTarget
		}
	}
	if e.xi != 0 {
		x := s.xtOf(e)
		m := uint8(0)
		for j := uint8(0); j < x.n; j++ {
			if x.slot[j] != uint8(i) || a0.Targets[x.slot[j]].TFlags&kop.TgfAtResolution != 0 {
				x.slot[m], x.code[m], x.ver[m] = x.slot[j], x.code[j], x.ver[j]
				m++
			}
		}
		x.n = m
	}
	e.tgt[i] = NoTarget
	for n, o := range picks {
		c := uint8(o)
		var ver uint8
		if isObjTarget(c) {
			ver = s.objs[c].ver
		} else if isStackTarget(c) {
			ver = s.stackTgtSid(c) // a stack ability is found again by its identity (tgt_stackability.go)
		}
		if n == 0 {
			e.tgt[i], e.tver[i] = c, ver
			continue
		}
		x := s.xtMut(e)
		if x == nil {
			return
		}
		if x.n < maxXTgt {
			x.slot[x.n], x.code[x.n], x.ver[x.n] = uint8(i), c, ver
			x.n++
		} else {
			s.capFull(StallXTgt)
		}
	}
	if e.nt <= uint8(i) {
		e.nt = uint8(i) + 1
	}
	if a0.Targets[i].Divide != 0 && len(picks) != oldN {
		// gorge's AnnouncedDivision requires len(Division) == len(Targets):
		// a count-changing retarget drops the inherited division, and damage
		// splits at resolution.
		if x := s.xtMut(e); x != nil {
			x.div = [maxXTgt + 1]uint8{}
		}
	}
	s.touch(e.obj)
}

// FeatCopyInherited is, at a spell copy's new-target election
// (SubCopyTarget), the index of target t among the copy's inherited
// targets, or -1 (gorge lists the inherited targets first, then the census
// in its usual order).
func (s *State) FeatCopyInherited(t uint8) int {
	if s.ctl != ctlResolve || s.res.ask != raGenCopyTarget && s.res.ask != raGenCopyTargets {
		return -1
	}
	var inh [MaxOptions]uint8
	n := s.copyInherited(&s.res.e, int(s.res.left), &inh)
	for i := 0; i < n; i++ {
		if inh[i] == t {
			return i
		}
	}
	return -1
}
