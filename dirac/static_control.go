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
//	r.src  the source object | its ver << 8 (NoObj: none); an object
//	       wider than a byte keeps r.src the object and r.p
//	       its ver
//	r.fl   unused; r.ts the CR 613.7a timestamp (static: the source's
//	       layerTs; API: s.ts at begin)

const (
	fcTo       = 1 << 0
	fcPrev     = 1 << 1
	fcYou      = 1 << 2
	fcDurShift = 3
	fcDurMask  = 7 << fcDurShift
	fcStatic   = 1 << 6
	fcArmed    = 1 << 7 // DurYourNextTurn: the controller's next turn is the current one or has begun
)

// fcHaste is fxRec.r.fl bit 7 of a control record: the record's object has
// haste while the record lives (gorge grants AddKWs$ for as long as the
// control effect lasts). fl's bits 0-5 are the multi-seat packing.
const fcHaste = 1 << 7

// ctrlKw is the keywords sl's live control records grant (GcHasteEOT).
func (s *State) ctrlKw(sl Slot) kop.KwMask {
	var m kop.KwMask
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxControl && r.r.fl&fcHaste != 0 {
			m |= kop.KwHaste
		}
	}
	return m
}

// ctrlKwSince is ctrlKw for a permanent that lost all its abilities at
// timestamp lts (CR 613.1f, 613.7): a control record's haste granted before
// the loss is removed by it, one granted at or after survives (gorge
// grantLostTo). lost false is ctrlKw.
func (s *State) ctrlKwSince(sl Slot, lts uint16, lost bool) kop.KwMask {
	if !lost {
		return s.ctrlKw(sl)
	}
	var m kop.KwMask
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxControl && r.r.fl&fcHaste != 0 && r.r.ts >= lts {
			m |= kop.KwHaste
		}
	}
	return m
}

// the pool's ended set is a uint64 bitmask. maxFx 16 -> 32: a declared
// capacity since 2026-10-27 (every Storm of Souls
// entrant takes its own override record); 32 -> 64 since 2026-11-03.
var _ = [1]struct{}{}[maxFx-64]

// At more than two seats a seat takes two bits: to, prev and you live in
// r.fl (bits 0-1, 2-3, 4-5; unused by a control record) and idx's bits 0-2
// stay clear.
func (r *fxRec) cTo() uint8 {
	if MaxSeats > 2 {
		return r.r.fl & 3
	}
	return r.r.idx & fcTo
}
func (r *fxRec) cPrev() uint8 {
	if MaxSeats > 2 {
		return r.r.fl >> 2 & 3
	}
	return r.r.idx & fcPrev >> 1
}
func (r *fxRec) cYou() uint8 {
	if MaxSeats > 2 {
		return r.r.fl >> 4 & 3
	}
	return r.r.idx & fcYou >> 2
}
func (r *fxRec) cDur() uint8 { return r.r.idx & fcDurMask >> fcDurShift }
func (r *fxRec) cSrc() Obj   { return Obj(r.r.src) }
func (r *fxRec) cSrcVer() uint8 {
	if NoObj > 0xff {
		return uint8(r.r.p)
	}
	return uint8(r.r.src >> 8)
}
func (r *fxRec) setPrev(p uint8) {
	if MaxSeats > 2 {
		r.r.fl = r.r.fl&^(3<<2) | (p&3)<<2
		return
	}
	r.r.idx = r.r.idx&^fcPrev | (p&1)<<1 // seat-ok: the two-seat packing; MaxSeats > 2 packs two bits in fl above
}

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
			if (in.Op == kop.OpOverride || in.Op == kop.OpOverrideAll) && in.A.K == kop.VConst {
				if in.A.N == kop.DurUntilYourNextTurn {
					g.anyNTOv = true
				}
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
	if MaxSeats > 2 {
		r.r.idx = (dur & 7) << fcDurShift
		r.r.fl = to&3 | (p.ctrl&3)<<2 | (you&3)<<4
	} else {
		r.r.idx = to&1 | (p.ctrl&1)<<1 | (you&1)<<2 | (dur&7)<<fcDurShift // seat-ok: the two-seat packing; MaxSeats > 2 packs two bits each in fl above
	}
	if static {
		r.r.idx |= fcStatic
		// CR 613.7a: the static's layer timestamp, captured at registration
		// (gorge controlGrant.orderStamp). ctrlStaticShadowed keeps an older
		// want from registering above this record.
		if int(src) < int(s.nobj) && s.objs[src].zone == kop.ZoneBattlefield && s.objs[src].ver == srcVer {
			r.r.ts = s.lstamp[s.objs[src].slot]
		}
	} else {
		// CR 613.7a: an API effect takes the layer clock when it began.
		r.r.ts = s.ts
	}
	if dur == kop.DurYourNextTurn && s.active != you {
		// made during another player's turn: the controller's next turn
		// is the next one they take
		r.r.idx |= fcArmed
	}
	if NoObj > 0xff {
		r.r.src, r.r.p = CardID(src), int16(srcVer)
	} else {
		r.r.src = CardID(src) | CardID(srcVer)<<8
	}
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
	if k.ascend {
		s.blessScan(k) // CR 702.131b (kw_ascend.go)
	}
	if k.gen != nil && k.gen.anyWhileHost {
		s.ovAsLongAsControlDrop(k, p.obj) // CR 611.2b: Pump Duration$ AsLongAsControl (filter_combatwords.go)
	}
	if k.gen != nil && k.gen.anyMayWhileSrc {
		s.mayWhileSrcCtrlDrop(p.obj, to) // CR 611.2b (cast_aslongascontrol.go)
	}
	if s.g.nul != 0 {
		s.untapLockCtrlDrop(p.obj, to) // CR 611.2b (act_untaplock.go)
	}
	if k.gen != nil && k.gen.anyEmbCtrl {
		s.embCtrlDrop(k, p.obj, to) // CR 611.2b: AsLongAsControl emblems (static_emblemdurations.go)
	}
	if k.gen != nil && k.gen.ringOn() {
		s.ringControlChanged(p.obj, to) // CR 701.54b: a Ring-bearer designation ends on a control change
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
			s.forgetBlocked(m.first(), sl)
		}
	}
	if p.flags&pfBlocking != 0 {
		s.releaseBlocked(sl)
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
	if MaxSeats > 2 && s.ctrlSeatLeft(r) {
		return true // CR 800.4a: an effect giving a departed seat control ends (zone_leavegame.go)
	}
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
// filter matches, and controlled by the record's controller.
func (s *State) ctrlStaticLive(k *prep, r *fxRec) bool {
	src := s.ctrlSrcSlot(r)
	if src == NoSlot || s.perms[src].attached != r.sl || s.perms[src].ctrl != r.cTo() {
		return false
	}
	return s.ctrlStaticWants(k, src, r.sl)
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
// whose source condition ended go, then every live control static registers
// its record, in CR 613.7a layer-timestamp order. A static that already
// names the host's controller still registers (a no-op record): it keeps
// its timestamp place below any later API control effect, so it cannot
// register late above that effect when a newer static ends. This is gorge's
// reconcileControlStatics (engine/rules/control_static.go).
//
// The pass repeats to a fixed point (CR 613.8a): registering a record can
// move its bearer, and a static on the moved bearer then wants a different
// controller — a Steal Enchantment that steals a second Steal Enchantment
// moves that Aura first, and the stolen Aura's own "you control enchanted
// permanent" applies with its NEW controller in the same check, moving the
// creature it holds. gorge reaches the same state across its emit tails (the
// next expireControl ends the stale grant, the next reconcile re-registers
// it); here the passes run back to back so the ask the resolution poses next
// sees the settled board. Each pass adds a record (a tracked want is
// skipped), removes one or moves a bearer, so the fixed point exists; the
// bound keeps a pathological loop finite, like ctrlExpire's.
//
//go:noinline
func (s *State) ctrlSBA(k *prep) {
	// The battlefield's controllers at entry: a pass that moves one changes
	// the affected set of every controller-keyed static (a global override
	// static like Ashaya's "nontoken creatures you control are Forest
	// lands"), whose cached override-derived types refreshTy must recompute
	// before the ask the caller poses next is projected. The baseline is over
	// every occupied slot, not just the record bearers: a pass can register a
	// bearer's FIRST-EVER control record (a control Aura on an unclaimed
	// permanent), and such a move ends at the baseline controller 0 for half
	// the steals, where a bearer-only baseline reads as no move (zero is also
	// a real controller). Control moves are rare, so the refresh is paid only
	// when one happened.
	var start [MaxPerm]uint8
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		start[sl] = s.perms[sl].ctrl
	}
	var prev [MaxPerm]uint8
	for pass := 0; pass <= maxFx; pass++ {
		nfx := s.g.nfx
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			prev[sl] = s.perms[sl].ctrl
		}
		if nfx > 0 {
			s.ctrlExpire(k, false)
		}
		if !s.ctrlRegister(k) {
			// StallTables: the table filled and s.stall ended the game, so
			// no ask follows and the refresh is dead work (the stalled path
			// stays as it was before the refresh gate existed).
			return
		}
		if s.g.nfx == nfx && !s.ctrlMoved(&prev) {
			break
		}
	}
	if s.ctrlMoved(&start) {
		s.refreshTy(k)
	}
}

// ctrlRegister registers every live StatControl static's record in CR 613.7a
// layer-timestamp order; false: the record table filled (StallTables).
func (s *State) ctrlRegister(k *prep) bool {
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.statM.and(s.occ), &ord)
	// Insertion sort: a handful of statics at most, and sort.Slice would
	// move ord to the heap (the gate's escape step).
	for i := 1; i < n; i++ {
		v := ord[i]
		vt := s.layerTs(v)
		j := i - 1
		for ; j >= 0; j-- {
			t := s.layerTs(ord[j])
			if t < vt || (t == vt && ord[j] < v) {
				break
			}
			ord[j+1] = ord[j]
		}
		ord[j+1] = v
	}
	for _, src := range ord[:n] {
		sp := &s.perms[src]
		host := sp.attached
		if host == NoSlot || !k.gen.has(s.cardAt(src), gfCtrlStat) {
			continue
		}
		o := sp.obj
		if s.ctrlStaticTracked(host, sp.ctrl, o, s.objs[o].ver) || !s.ctrlStaticWants(k, src, host) {
			continue
		}
		// CR 613.7: a record already tracked on this host with a later
		// timestamp wins; this want stays live but must not register above
		// it. Without this an older Aura whose controller changes would
		// re-register at the end of the record list, above a newer effect
		// (gorge staticGrantNewer).
		if s.ctrlStaticShadowed(host, src, s.layerTs(src)) {
			continue
		}
		if !s.gainControl(host, sp.ctrl, sp.ctrl, kop.DurPermanent, o, s.objs[o].ver, true) {
			return false
		}
	}
	return true
}

// ctrlMoved reports whether any occupied slot changed hands during the pass
// (the record table is the same size but a bearer moved, so the next pass must
// re-derive the wants that read it). It reads every occupied slot, not just
// those still in fxM: ctrlExpire rebuilds fxM from the surviving records
// before its revert moves the controller, so a slot whose only record ended
// this pass has left fxM by the time it is compared.
func (s *State) ctrlMoved(prev *[MaxPerm]uint8) bool {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].ctrl != prev[sl] {
			return true
		}
	}
	return false
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

// ctrlStaticShadowed reports whether a control record already tracked on host
// has a CR 613.7 timestamp at or after the static at src (gorge
// staticGrantNewer). Such a want is shadowed: it stays live but must not
// register above the tracked record. The timestamp is the one the record
// locked at registration (gainControl). An equal-stamp API record shadows
// (it is already registered, so it began first); an equal-stamp static record
// shadows only a higher source slot, the order ctrlSBA registers equal stamps
// in.
func (s *State) ctrlStaticShadowed(host, src Slot, stamp uint16) bool {
	if !s.fxM.has(host) {
		return false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind != fxControl || r.sl != host {
			continue
		}
		if r.r.idx&fcStatic == 0 {
			if r.r.ts >= stamp {
				return true
			}
			continue
		}
		rs := s.ctrlSrcSlot(r)
		if rs == NoSlot {
			continue
		}
		if r.r.ts > stamp || (r.r.ts == stamp && rs > src) {
			return true
		}
	}
	return false
}

// opGainControl is OpGainControl's resolution.
//
//go:noinline
func (s *State) opGainControl(k *prep, in *kop.Instr) {
	var list [MaxPerm]Slot
	n := 0
	if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
		// Defined$ over a remembered group (Fumble's "gain control of all
		// Auras and Equipment that were attached to it"): each battlefield
		// member in remembered order, one effect apiece (gorge effGainControl
		// walks its Defined set)
		for _, o := range s.g.rem[:s.g.nrem] {
			if o != NoObj && s.objs[o].zone == kop.ZoneBattlefield && n < len(list) {
				list[n] = s.objs[o].slot
				n++
			}
		}
	} else if in.Obj == kop.RefNone && in.F != 0 {
		n = s.gainControlAllHits(k, in.F, &list) // AllValid$ (act_gaincontrolall.go)
	} else if sl := s.objRef(k, in.Obj); sl != NoSlot {
		list[0], n = sl, 1
	}
	if n == 0 {
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
	for _, sl := range list[:n] {
		s.gainControlOne(k, in, sl, to, dur, src, ver)
	}
}

// gainControlOne is opGainControl's grant to the permanent at sl, with its
// Untap$ / haste / RememberControlled$ riders (gorge effGainControl's
// per-target body).
func (s *State) gainControlOne(k *prep, in *kop.Instr, sl Slot, to, dur uint8, src Obj, ver uint8) {
	s.touchSl(sl)
	if !s.gainControl(sl, to, s.res.e.ctrl, dur, src, ver, false) {
		return
	}
	p := &s.perms[sl]
	if in.B.N&kop.GcUntap != 0 {
		if p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
			p.seeCtr(s)
			s.subCtrFire(k, sl, int(kop.CtrStun), 1) // CR 122.1d (gorge TryUntap)
		} else if !s.untapBlocked(k, sl) {
			s.untapEffect(k, sl)
		}
	}
	if in.B.N&kop.GcHasteEOT != 0 {
		// AddKWs$: gorge grants it for as long as the control effect lasts
		// (dropControlKeywords), so the record carries it at every duration;
		// an until-end-of-turn steal also ends early when its controller leaves
		// the game (CR 800.4a; ctrlSeatLeft), taking the haste with it
		s.g.fx[s.g.nfx-1].r.fl |= fcHaste // the record gainControl just appended
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
		return s.nextAlive(e.ctrl), true // the first of gorge's Opponent list (oneOpp)
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		t := s.tgtCode(sl)
		if isPlayerTarget(t) {
			return CodeSeat(t), true
		}
	case kop.RefTriggerPlayer:
		// NewController$ TriggeredTarget / TriggeredActivator: the player
		// the trigger named (gorge definedSpecTriggeredPlayer)
		if e.trigPl != 255 {
			return e.trigPl, true
		}
	case kop.RefTriggerTgtCtrl:
		// NewController$ TriggeredSourceController: the trigger object's
		// controller read live (playerRef)
		if o := e.trigObj; o != NoObj {
			return s.ctrlOfObj(o), true
		}
	case kop.RefChosenPlayer:
		// NewController$ ChosenPlayer: the opponent this resolution's
		// ChoosePlayer chose (Akroan Horse, Wishclaw Talisman; gorge
		// controlPlayer's Defined read of Ctx.Chosen)
		if DebugChosenNextSeat {
			return s.nextAlive(e.ctrl), true // seat-ok: a test-only mutation
		}
		if p := s.chosenPlayerSeat(e); p != 255 {
			return p, true
		}
	}
	return 0, false
}

// nextActive is the player whose turn follows this one: the next pending
// grant's seat, else the ordinary rotation (CR 500.7; act_extraturn.go).
func (s *State) nextActive() uint8 {
	return s.exNextTurn()
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
		s.ovNTArm(s.res.e.ctrl) // static_ovntleave.go
		return ovNT | (s.res.e.ctrl<<ovNTWhoShift)&ovNTWho
	case kop.DurUntilYourNextUpkeep:
		return 0 // the class is the Override row's UntilUpkeep; p carries the controller (static_untilnextupkeep.go)
	}
	return 0
}

// dropNextTurnOv drops the override records lasting until their
// controller's next turn when that turn is the next one: gorge ends an
// UntilYourNextTurn effect at the cleanup before the turn begins
// (layers_effect.go: UntilTurn = nextTurnFor - 1).
func (s *State) dropNextTurnOv() (ended slotMask) {
	var have uint8
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.kind == fxOverride && r.r.fl&ovNT != 0 {
			have |= 1 << ((r.r.fl & ovNTWho) >> ovNTWhoShift)
		}
	}
	ends := s.ovNTEnding(have) // static_ovntleave.go: a controller that left ends at its frozen boundary
	n := 0
	s.fxM = slotMask{}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind == fxOverride && r.r.fl&ovNT != 0 && ends&(1<<((r.r.fl&ovNTWho)>>ovNTWhoShift)) != 0 {
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
