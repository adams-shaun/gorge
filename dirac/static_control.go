package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 613 layer 2 (control-changing effects, CR 613.1b).
//
// Every control-changing effect is a fxControl record in the effect pool
// (g.fx), appended in the order the effects began, so pool order is
// timestamp order (CR 613.7) within layer 2. Control is settled eagerly:
// the permanent's ctrl/ctrlM are written when a record is added or ends,
// never derived on a read, so every legality and target check keeps
// reading ctrlM as before.
//
// This is a port of gorge's control tracker (engine/rules/control.go,
// RegisterControl / expireControl, and control_static.go's reconcile for
// an S:Mode$ Continuous GainControl$ static):
//
//   - a record holds the controller immediately before the effect (prev)
//     and the controller it gives (to); a change of controller is recorded
//     even when it moves nothing (gorge RegisterControl always appends: as
//     the latest effect it keeps an earlier effect's end from moving
//     control);
//   - when records end, each affected object goes to the latest surviving
//     record's controller, or to the earliest record's prev when none
//     survives; the earliest survivor inherits that prev; control moves
//     only when the object's newest record ended and the object is still
//     under that record's controller;
//   - a record of an object that left the battlefield is dropped without a
//     change (clearFx: the object is new, CR 400.7);
//   - the loop repeats, because a returned permanent can be the source of
//     another record (LoseControl$ LoseControl).
//
// Record layout (fxRec.r reused, 8 bytes, no State growth):
//
//	r.idx  bit 0 to, bit 1 prev, bit 2 you (the effect's controller),
//	       bits 3-5 the duration (kop.Dur*), bit 6 static, bit 7 armed
//	r.src  the source object | its ver << 8 (NoObj: none)
//	r.fl   unused; r.ts 0

const (
	fcTo       = 1 << 0
	fcPrev     = 1 << 1
	fcYou      = 1 << 2
	fcDurShift = 3
	fcDurMask  = 7 << fcDurShift
	fcStatic   = 1 << 6
	fcArmed    = 1 << 7 // DurYourNextTurn: the controller's next turn is the current one or has begun
)

// the pool's ended set is a uint64 bitmask. maxFx 16 -> 32: a declared
// capacity since 2026-10-27 (every Storm of Souls
// entrant takes its own override record); 32 -> 64 since 2026-11-03.
var _ = [1]struct{}{}[maxFx-64]

func (r *fxRec) cTo() uint8   { return r.r.idx & fcTo }
func (r *fxRec) cPrev() uint8 { return r.r.idx & fcPrev >> 1 }
func (r *fxRec) cYou() uint8  { return r.r.idx & fcYou >> 2 }
func (r *fxRec) cDur() uint8  { return r.r.idx & fcDurMask >> fcDurShift }
func (r *fxRec) cSrc() Obj    { return Obj(r.r.src) }
func (r *fxRec) cSrcVer() uint8 {
	return uint8(r.r.src >> 8)
}
func (r *fxRec) setPrev(p uint8) { r.r.idx = r.r.idx&^fcPrev | (p&1)<<1 }

// prepCtrl derives the control gates: ctrlStat per card (a StatControl
// static) and ctrlSBA (the state-based check must look at control: a
// control static, or an OpGainControl whose duration reads its source).
func prepCtrl(t *kop.Table, g *genPrep) {
	scan := func(prog []kop.Instr) {
		for i := range prog {
			in := &prog[i]
			if in.Op == kop.OpGainControl && (in.A.N == kop.DurWhileSource || in.A.N == kop.DurWhileControlSource) {
				g.ctrlSBA = true
			}
			if (in.Op == kop.OpOverride || in.Op == kop.OpOverrideAll) && in.A.K == kop.VConst && in.A.N == kop.DurUntilYourNextTurn {
				g.anyNTOv = true
			}
		}
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatControl {
				g.setFlag(CardID(i), gfCtrlStat)
				g.ctrlSBA = true
			}
			scan(a.Prog)
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			scan(t.Emblems[i].Abilities[j].Prog)
		}
	}
}

// gainControl is OpGainControl / OpGainControlEOT on the permanent at sl:
// player to gains control of it for duration dur (CR 613.1b). you is the
// effect's controller; src/srcVer the source object (NoObj: none); static
// marks a record a StatControl static registered.
//
//go:noinline
func (s *State) gainControl(sl Slot, to, you, dur uint8, src Obj, srcVer uint8, static bool) bool {
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return false
	}
	p := &s.perms[sl]
	r := fxRec{sl: sl, kind: fxControl}
	r.r.idx = to&1 | (p.ctrl&1)<<1 | (you&1)<<2 | (dur&7)<<fcDurShift
	if static {
		r.r.idx |= fcStatic
	}
	if dur == kop.DurYourNextTurn && s.active != you {
		// made during another player's turn: the controller's next turn
		// is the next one they take
		r.r.idx |= fcArmed
	}
	r.r.src = CardID(src) | CardID(srcVer)<<8
	s.g.fx[s.g.nfx] = r
	s.g.nfx++
	s.fxM.set(sl)
	if p.ctrl != to {
		s.ctrlMove(sl, to)
		p.flags |= pfCtrlEOT
	}
	return true
}

// ctrlMove gives the permanent at sl to player to (gorge changeControl):
// it rejoins the end of that player's battlefield list, leaves combat and
// is summoning sick (CR 302.6, 506.4).
func (s *State) ctrlMove(sl Slot, to uint8) {
	p := &s.perms[sl]
	if p.ctrl == to {
		return
	}
	s.touchSl(sl)
	if p.flags&(pfAttacking|pfBlocking) != 0 {
		s.ctrlUncombat(sl)
	}
	s.ctrlM[p.ctrl].clr(sl)
	p.ctrl = to
	s.ctrlM[to].set(sl)
	s.joined(p)
	p.flags |= pfSick
	k := s.k()
	if k.engines {
		s.speedControl(k) // kw_speed.go
	}
	if k.gen != nil && k.gen.anyMayWhileSrc {
		s.mayWhileSrcCtrlDrop(p.obj, to) // CR 611.2b (cast_aslongascontrol.go)
	}
}

// ctrlUncombat removes the permanent at sl from combat (gorge
// changeControl: IsAttacking/IsBlocking cleared, it is no blocker of any
// attacker any more; an attacker it blocked stays blocked, CR 506.4).
func (s *State) ctrlUncombat(sl Slot) {
	p := &s.perms[sl]
	if p.flags&pfAttacking != 0 {
		s.nAttackers--
		for m := s.occ; m.any(); m = m.rest() {
			if q := m.first(); s.perms[q].blocking == sl {
				s.perms[q].blocking = NoSlot
				s.touchSl(q)
			}
		}
	}
	if p.flags&pfBlocking != 0 && p.blocking != NoSlot {
		if a := &s.perms[p.blocking]; a.nblock > 0 {
			a.nblock--
			s.touchSl(p.blocking)
		}
	}
	p.flags &^= pfAttacking | pfBlocking | pfBlocked
	p.blocking = NoSlot
	p.nblock = 0
}

// ctrlSrcSlot is the battlefield slot of r's source while it is the same
// object (NoSlot otherwise).
func (s *State) ctrlSrcSlot(r *fxRec) Slot {
	o := r.cSrc()
	if o == NoObj || int(o) >= int(s.nobj) {
		return NoSlot
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.ver != r.cSrcVer() {
		return NoSlot
	}
	return ob.slot
}

// ctrlEnded reports whether control record r is over (gorge grantEnded).
func (s *State) ctrlEnded(k *prep, r *fxRec, cleanup bool) bool {
	if r.r.idx&fcStatic != 0 {
		return !s.ctrlStaticLive(k, r)
	}
	switch r.cDur() {
	case kop.DurEOT:
		return cleanup
	case kop.DurYourNextTurn:
		return cleanup && s.active == r.cYou() && r.r.idx&fcArmed != 0
	case kop.DurWhileSource:
		return s.ctrlSrcSlot(r) == NoSlot
	case kop.DurWhileControlSource:
		src := s.ctrlSrcSlot(r)
		return src == NoSlot || s.perms[src].ctrl != r.cYou()
	}
	return false
}

// ctrlStaticLive: the static that registered r still applies to its
// object with the same controller (gorge staticGrantLive): the same source
// on the battlefield with its abilities, attached to the object, which its
// filter matches, and controlled by the record's controller. It also must
// still be the layer-2 outcome for the object: a control static with a
// later layer timestamp that gives the object to a different controller
// overrides it even though its own static is still live (CR 613.1b/613.7).
func (s *State) ctrlStaticLive(k *prep, r *fxRec) bool {
	src := s.ctrlSrcSlot(r)
	if src == NoSlot || s.perms[src].attached != r.sl || s.perms[src].ctrl != r.cTo() {
		return false
	}
	if !s.ctrlStaticWants(k, src, r.sl) {
		return false
	}
	_, winCtrl, ok := s.ctrlStaticWinner(k, r.sl)
	return ok && winCtrl == r.cTo()
}

// ctrlStaticWinner returns the control static whose effect wins layer 2 for
// the permanent at host: the live static source with the latest layer
// timestamp (gorge staticControlLayer, CR 613.1b/613.7). Ties keep the
// wanted-set scan order, so the last source in it wins.
func (s *State) ctrlStaticWinner(k *prep, host Slot) (Slot, uint8, bool) {
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.statM.and(s.occ), &ord)
	win, winCtrl := NoSlot, uint8(0)
	var winTs uint16
	for _, src := range ord[:n] {
		sp := &s.perms[src]
		if sp.attached != host || !k.gen.has(s.cardAt(src), gfCtrlStat) {
			continue
		}
		if !s.ctrlStaticWants(k, src, host) {
			continue
		}
		if ts := s.layerTs(src); win == NoSlot || ts >= winTs {
			win, winTs, winCtrl = src, ts, sp.ctrl
		}
	}
	return win, winCtrl, win != NoSlot
}

// ctrlStaticWants: the source at src has a live StatControl static whose
// filter matches the permanent at sl.
func (s *State) ctrlStaticWants(k *prep, src, sl Slot) bool {
	cid := s.cardAt(src)
	if !k.gen.has(cid, gfCtrlStat) || s.genLost(k, src) || !s.occ.has(sl) {
		return false
	}
	cd := &k.t.Cards[cid]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatControl &&
			s.matchPermGen(k, a.Static.Filter, sl, s.perms[src].ctrl, src, layerFull) {
			return true
		}
	}
	return false
}

// ctrlExpire ends the control records that are over and moves control as
// the remaining records say (gorge expireControl). cleanup: the cleanup
// step's turn-structure ends (CR 514.2) are being passed.
//
//go:noinline
func (s *State) ctrlExpire(k *prep, cleanup bool) {
	for pass := 0; pass <= maxFx+1; pass++ {
		var ended uint64
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.kind == fxControl && s.ctrlEnded(k, r, cleanup) {
				ended |= 1 << uint(i)
			}
		}
		if ended == 0 {
			return
		}
		var chSl [maxFx]Slot
		var chTo [maxFx]uint8
		nch := 0
		var done slotMask
		for i := 0; i < int(s.g.nfx); i++ {
			r := &s.g.fx[i]
			if r.kind != fxControl || done.has(r.sl) {
				continue
			}
			sl := r.sl
			done.set(sl)
			base := r.cPrev()
			last, first, lastSurv := -1, -1, -1
			for j := i; j < int(s.g.nfx); j++ {
				if q := &s.g.fx[j]; q.kind == fxControl && q.sl == sl {
					last = j
					if ended&(1<<j) == 0 {
						if first < 0 {
							first = j
						}
						lastSurv = j
					}
				}
			}
			want := base
			if lastSurv >= 0 {
				want = s.g.fx[lastSurv].cTo()
				s.g.fx[first].setPrev(base)
			}
			if ended&(1<<last) != 0 && s.perms[sl].ctrl == s.g.fx[last].cTo() && want != s.perms[sl].ctrl {
				chSl[nch], chTo[nch] = sl, want
				nch++
			}
		}
		n := 0
		s.fxM = slotMask{}
		for i := 0; i < int(s.g.nfx); i++ {
			if ended&(1<<i) != 0 {
				continue
			}
			s.g.fx[n] = s.g.fx[i]
			s.fxM.set(s.g.fx[n].sl)
			n++
		}
		for i := n; i < int(s.g.nfx); i++ {
			s.g.fx[i] = fxRec{}
		}
		s.g.nfx = uint8(n)
		for i := 0; i < nch; i++ {
			s.ctrlMove(chSl[i], chTo[i])
		}
		s.ctrlFlags(done)
		cleanup = false // the fixed point has been passed
	}
}

// ctrlFlags recomputes pfCtrlEOT (a record that changed control is on the
// slot) for the slots in m.
func (s *State) ctrlFlags(m slotMask) {
	for ; m.any(); m = m.rest() {
		sl := m.first()
		on := false
		if s.fxM.has(sl) {
			for i := 0; i < int(s.g.nfx); i++ {
				if r := &s.g.fx[i]; r.kind == fxControl && r.sl == sl && r.cTo() != r.cPrev() {
					on = true
					break
				}
			}
		}
		p := &s.perms[sl]
		if !on && p.flags&pfCtrlEOT != 0 {
			p.flags &^= pfCtrlEOT
			s.touchSl(sl)
		}
	}
}

// ctrlCleanup is the cleanup step's control expiry (CR 514.2: until end of
// turn; the end of the controller's next turn), then arms the next-turn
// records made during this turn of their controller.
func (s *State) ctrlCleanup(k *prep) {
	s.ctrlExpire(k, true)
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.kind == fxControl && r.cDur() == kop.DurYourNextTurn && r.cYou() == s.active {
			r.r.idx |= fcArmed
		}
	}
}

// ctrlSBA is the control part of the state-based check (gorge runs
// expireControl and reconcileControlStatics after every event): records
// whose source condition ended go, then the layer-2 winner among the live
// control statics registers its record, a no-op one included when its
// controller already controls the permanent (gorge's noop grant: it still
// carries the effect's timestamp). Control-changing effects in layer 2 are
// not additive (CR 613.1b/613.7): only the latest-timestamp static has any
// effect, so an older static must not re-register after a newer one took
// the permanent.
//
// The statics are walked in gorge's wanted-set order -- player 0's
// battlefield list, then player 1's (control_static.go staticControlWants
// preserves the static_layerwalk.go staticEffects walk) -- not slot order:
// enterNF reuses the lowest free slot, so a later-entered Aura can hold a
// lower slot than an older one. staticControlLayer keeps the want with the
// latest effectStamp and never reorders, so on a tie the last source in
// that scan order wins (the >= below).
//
//go:noinline
func (s *State) ctrlSBA(k *prep) {
	if s.g.nfx > 0 {
		s.ctrlExpire(k, false)
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.statM.and(s.occ), &ord)
	if n == 0 {
		return
	}
	var winSrc [MaxPerm]Slot
	var winTs [MaxPerm]uint16
	var have slotMask
	for _, src := range ord[:n] {
		sp := &s.perms[src]
		host := sp.attached
		if host == NoSlot || !k.gen.has(s.cardAt(src), gfCtrlStat) {
			continue
		}
		if !s.ctrlStaticWants(k, src, host) {
			continue
		}
		if ts := s.layerTs(src); !have.has(host) || ts >= winTs[host] {
			have.set(host)
			winTs[host] = ts
			winSrc[host] = src
		}
	}
	for _, src := range ord[:n] {
		sp := &s.perms[src]
		host := sp.attached
		if host == NoSlot || !have.has(host) || winSrc[host] != src {
			continue
		}
		// Register even when the winner's controller already controls the
		// host (gorge reconcileControlStatics's noop grant): gainControl
		// makes a same-controller record a no-op move, and recording it is
		// what stops the standing static from re-stealing the host after a
		// later control change (ctrlStaticTracked guards duplicates by
		// source and version).
		o := sp.obj
		if s.ctrlStaticTracked(host, sp.ctrl, o, s.objs[o].ver) {
			continue
		}
		if !s.gainControl(host, sp.ctrl, sp.ctrl, kop.DurPermanent, o, s.objs[o].ver, true) {
			return
		}
	}
}

// ctrlStaticTracked: the static of source o (ver) already has its record
// giving sl to player to (gorge staticGrantTracked: a newer steal on top of
// a standing static grant does not make it register again).
func (s *State) ctrlStaticTracked(sl Slot, to uint8, o Obj, ver uint8) bool {
	if !s.fxM.has(sl) {
		return false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.kind == fxControl && r.sl == sl && r.r.idx&fcStatic != 0 && r.cTo() == to && r.cSrc() == o && r.cSrcVer() == ver {
			return true
		}
	}
	return false
}

// opGainControl is OpGainControl's resolution.
//
//go:noinline
func (s *State) opGainControl(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	e := &s.res.e
	to := e.ctrl
	if in.C != int16(kop.RefNone) {
		p, ok := s.ctrlPlayerRef(kop.Ref(in.C))
		if !ok {
			return
		}
		to = p
	}
	dur := uint8(in.A.N)
	src, ver := NoObj, uint8(0)
	if dur == kop.DurWhileSource || dur == kop.DurWhileControlSource {
		// CR 611.2b: a "for as long as" duration that has already ended
		// when the effect would begin does nothing (gorge ControlGrantEnded
		// before RegisterControl)
		ssl := s.objRef(k, kop.RefSelf)
		if ssl == NoSlot || dur == kop.DurWhileControlSource && s.perms[ssl].ctrl != e.ctrl {
			return
		}
		src = s.perms[ssl].obj
		ver = s.objs[src].ver
	}
	s.touchSl(sl)
	if !s.gainControl(sl, to, e.ctrl, dur, src, ver, false) {
		return
	}
	p := &s.perms[sl]
	if in.B.N&kop.GcUntap != 0 {
		if p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
			p.seeCtr(s)
			p.subCtrN(s, int(kop.CtrStun), 1) // CR 122.1d (gorge TryUntap)
		} else if !s.untapBlocked(k, sl) {
			s.untapEffect(k, sl)
		}
	}
	if in.B.N&kop.GcHasteEOT != 0 {
		s.grantEOT(sl, kop.KwHaste)
	}
	if in.B.N&kop.GcRemember != 0 {
		s.remember(p.obj) // RememberControlled$ (gorge effGainControl)
	}
}

// ctrlPlayerRef resolves OpGainControl's new-controller Ref (a player).
func (s *State) ctrlPlayerRef(r kop.Ref) (uint8, bool) {
	e := &s.res.e
	switch r {
	case kop.RefController:
		return e.ctrl, true
	case kop.RefOpponent, kop.RefEachOpponent:
		return 1 - e.ctrl, true
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		t := s.tgtCode(sl)
		if t >= PlayerTarget0 && t <= PlayerTarget1 {
			return uint8(t - PlayerTarget0), true
		}
	}
	return 0, false
}

// nextActive is the player whose turn follows this one (an extra turn
// first).
func (s *State) nextActive() uint8 {
	if s.pl[s.active].extraTurns > 0 {
		return s.active
	}
	return 1 - s.active
}

// ovDurFl is an OpOverride / OpOverrideAll duration's record bits.
func (s *State) ovDurFl(dur int16) uint8 {
	if pumpNTEndsEOT(dur) { // DebugPumpNTEndsEOT (static_untilnextturn.go)
		return ovEOT
	}
	switch dur {
	case kop.DurEOT:
		return ovEOT
	case kop.DurUntilYourNextTurn:
		return ovNT | (s.res.e.ctrl&1)*ovNTYou
	}
	return 0
}

// dropNextTurnOv drops the override records lasting until their
// controller's next turn when that turn is the next one: gorge ends an
// UntilYourNextTurn effect at the cleanup before the turn begins
// (layers_effect.go: UntilTurn = nextTurnFor - 1).
func (s *State) dropNextTurnOv() (ended slotMask) {
	nt := s.nextActive()
	n := 0
	s.fxM = slotMask{}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind == fxOverride && r.r.fl&ovNT != 0 && (r.r.fl&ovNTYou)>>5 == nt {
			s.touchSl(r.sl)
			ended = ended.with(r.sl)
			continue
		}
		s.g.fx[n] = *r
		s.fxM.set(r.sl)
		n++
	}
	for i := n; i < int(s.g.nfx); i++ {
		s.g.fx[i] = fxRec{}
	}
	s.g.nfx = uint8(n)
	return ended
}
