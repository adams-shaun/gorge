package dirac

import (
	"math"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// RefOppNonRemCtrl and RefSrcOwner (gorge effects
// context.go definedSpecNonRememberedController / definedSpecCardOwner).

// DebugOppNonRemFirstOnly is a test hook: oppsNonRemCtrl ignores the
// remembered set and always names the opponent. The lockstep must fail with
// it set.
var DebugOppNonRemFirstOnly bool

// oppsNonRemCtrl is RefOppNonRemCtrl: the controller's opponents in gorge's
// order (AliveFrom the controller), less every remembered object's controller
// (its LKI capture once it left the battlefield, else the live one). Every
// remembered object is tested, not just the first.
//
//go:noinline
func (s *State) oppsNonRemCtrl() seatSeq {
	q := s.oppsFrom(s.res.e.ctrl)
	if DebugOppNonRemFirstOnly {
		return q
	}
	var ex uint8 // seat mask of the remembered controllers
	for i := uint8(0); i < s.g.nrem; i++ {
		o := s.g.rem[i]
		c := uint8(0)
		if l := s.remLKIRec(o); l != nil {
			c = l.ctrl
		} else {
			c = s.ctrlOfObj(o)
		}
		ex |= 1 << SeatIx(c)
	}
	var out seatSeq
	for i := uint8(0); i < q.n; i++ {
		if ex&(1<<q.s[i]) == 0 {
			out.add(q.s[i])
		}
	}
	return out
}

// srcOwner is RefSrcOwner: the resolving source object's owner.
//
//go:noinline
func (s *State) srcOwner() uint8 {
	return s.objs[s.res.e.obj].owner()
}

// Cold readers of the grammar sweep (ConditionDefined$ Sacrificed).

// sacrificedObj is ConditionDefined$ Sacrificed + ConditionPresent$ f
// (gorge conditionMet, defined == "Sacrificed"): the group is the entry's
// sacrificed cost object, matched as the card it is NOW (its current face,
// CR 608.2h's object identity), not by its last known characteristics. A
// ceased token is no exception: gorge's g.Obj still returns the ZCeased
// object (an arena tombstone that keeps its face), so its card is matched
// where it lies (countRefMatches' one(), a card outside the battlefield).
// NoObj without a snapshot record (the cost held no sacrifice), or when the
// token slot was recycled since the snapshot.
//
//go:noinline
func (s *State) sacrificedObj(ctx evalCtx) Obj {
	e := ctx.e
	if e == nil || e.li == 0 || DebugSacrificedNone {
		return NoObj
	}
	r := s.lk(e.li)
	o := r.obj
	// a sacrificed token ceased at ver+1 and objReferenced keeps its slot
	// from being recycled while the entry holds the record; a token slot at
	// any other ver is a different token (defence in depth)
	if o == NoObj || (DebugSacrificedTokenSkip && s.objs[o].zone == kop.ZoneNone) || (s.objs[o].of&ofToken != 0 && s.objs[o].ver != r.ver+1) {
		return NoObj
	}
	return o
}

// DebugPumpEachFirst doubles every creature by the first creature's power
// and toughness (the mutation: gorge reads each creature's own).
var DebugPumpEachFirst bool

// DebugSacrificedTokenSkip skips a ceased sacrificed token (the mutation: the
// earlier reading that gorge's g.Obj no longer finds it).
var DebugSacrificedTokenSkip bool

// DebugPlayerPredNone ignores a Player.<q> set's predicate.
var DebugPlayerPredNone bool

// DebugSacrificedNone makes the Sacrificed condition group empty (the
// mutation test: the gate never opens).
var DebugSacrificedNone bool

// DebugDefinedHits, when set, counts the branches the lifts
// take, for the oracle's lockstep occurrence minima. Nil in production (every
// increment tests the pointer first); only a single-goroutine test sets it.
var DebugDefinedHits *[DefinedNumHits]uint32

// DebugDefinedSac, when set, observes one Sacrificed condition read: the
// entry's card, whether the sacrificed object is a token, and whether it
// matched the filter.
var DebugDefinedSac func(card CardID, token, matched bool)

// kdHit counts branch i when the test hook is set.
func kdHit(i int) {
	if h := DebugDefinedHits; h != nil {
		h[i]++
	}
}

// The KdefinedHits indices.
const (
	// (the Sacrificed condition reads go through DebugDefinedSac; the three
	// slots stay reserved so the predicate indices do not move)
	KdSacTrue = iota
	KdSacFalse
	KdSacCeased
	// KdPredBase starts the player-predicate counters: per predicate kind
	// (controlsCreature, MaxSpeed, IsCorrupted) four slots: the controller
	// kept / dropped, the opponent kept / dropped.
	KdPredBase
	// the Defined$ Valid sweeps (A): the sweep resolved over two
	// or more objects, per op
	KdPumpSweep    = KdPredBase + 16 // OpPumpAllEach with constant amounts
	KdPumpEach     = KdPumpSweep + 1 // OpPumpAllEach, an amount read per creature
	KdPumpEachDiff = KdPumpSweep + 2 // ... over creatures of different powers
	KdPumpEmpty    = KdPumpSweep + 3 // OpPumpAllEach over no creature
	KdPutSweep     = KdPumpSweep + 4 // OpPutCounterAll lowered from Defined$ Valid
	KdDoubleSweep  = KdPumpSweep + 5 // OpDoubleCountersAll
	KdSweepEmpty   = KdPumpSweep + 6 // OpDoubleCountersAll or the put over no permanent
	DefinedNumHits = KdPumpSweep + 7
)

// kdefHit records which branch a Sacrificed condition read took.
//
//go:noinline
func (s *State) kdefHit(ctx evalCtx, o Obj, n int) {
	if DebugDefinedSac == nil || ctx.e == nil || ctx.e.li == 0 || o == NoObj {
		return
	}
	DebugDefinedSac(ctx.e.card, s.objs[o].of&ofToken != 0, n > 0)
}

// playerPred is Player.<q> / Opponent.<q> for player p (gorge
// MatchesPlayerSpecWithSVars): the candidate controls a creature matching
// in.F (the object filter reads the resolving controller as "you"), has max
// speed, is corrupted, or controls the most creatures (a tie keeps both);
// PlayerPredNeg inverts it.
//
//go:noinline
func (s *State) playerPred(k *prep, in *kop.Instr, p uint8) bool {
	if DebugPlayerPredNone {
		return true
	}
	n := in.B.N
	r := false
	switch n &^ kop.PlayerPredNeg {
	case kop.PlayerPredControlsCreature:
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			if s.matchPermGen(k, in.F, m.first(), s.res.e.ctrl, NoSlot, layerFull) {
				r = true
				break
			}
		}
	case kop.PlayerPredArtOrEnch:
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			if s.typesAt(k, m.first())&(kop.TArtifact|kop.TEnchantment) != 0 {
				r = true
				break
			}
		}
	case kop.PlayerPredMaxSpeed:
		r = s.speedOf(p) >= speedMaxV
	case kop.PlayerPredCorrupted:
		r = s.g.poison[p] >= 3
	case kop.PlayerPredMostCreatures:
		r = s.mostCreatures(k, p)
	}
	kept := r == (n&kop.PlayerPredNeg == 0)
	if n&^kop.PlayerPredNeg > kop.PlayerPredArtOrEnch {
		return kept // the hit table covers the first four kinds
	}
	i := KdPredBase + int(n&^kop.PlayerPredNeg-1)*4 // 4 kinds
	if p != s.res.e.ctrl {
		i += 2
	}
	if !kept {
		i++
	}
	if i < DefinedNumHits {
		kdHit(i)
	}
	return kept
}

// mostCreatures reports whether p controls at least as many creatures as
// each other player still in the game (gorge playerHasMost: no player has
// more).
func (s *State) mostCreatures(k *prep, p uint8) bool {
	if MaxSeats == 2 {
		return s.creaturesCtrl(k, p) >= s.creaturesCtrl(k, 1-p)
	}
	n := s.creaturesCtrl(k, p)
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != p && !s.out(q) && s.creaturesCtrl(k, q) > n {
			return false
		}
	}
	return true
}

// creaturesCtrl is the number of creatures p controls (gorge playerHasMost
// TypeCreature's per-seat battlefield count).
func (s *State) creaturesCtrl(k *prep, p uint8) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if s.isCreature(k, m.first()) {
			n++
		}
	}
	return n
}

// playerSet filters an each-player op's candidates q by its predicate, every
// one read before any acts, keeping q's order.
//
//go:noinline
func (s *State) playerSet(k *prep, in *kop.Instr, q seatSeq) seatSeq {
	var o seatSeq
	for i := uint8(0); i < q.n; i++ {
		if s.playerPred(k, in, q.s[i]) {
			o.add(q.s[i])
		}
	}
	return o
}

// playerSetMask is playerSet for the each-player damage sequence: bit 0 the
// active player, bit 1 the other (damageSeqNext's dmgSeqPl); at more seats
// the seat mask of the seats still in the game that match.
//
//go:noinline
func (s *State) playerSetMask(k *prep, in *kop.Instr) uint8 {
	if MaxSeats != 2 {
		return seatBits(s.playerSet(k, in, s.aliveFrom(s.res.e.ctrl)))
	}
	var m uint8
	if s.playerPred(k, in, s.active) {
		m |= 1
	}
	if s.playerPred(k, in, 1-s.active) {
		m |= 2
	}
	return m
}

// execDefined runs the A sweeps (the tail of execDread's
// chain); done=false means the op is not one of these.
//
//go:noinline
func (s *State) execDefined(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpPumpAllEach:
		s.pumpAllEach(k, in)
	case kop.OpDoubleCountersAll:
		s.doubleCountersAll(k, in)
	default:
		return s.execPrepare(k, in) // face_prepare.go
	}
	return true, true
}

// eotAdd adds an end-of-turn pump to sl; an amount that leaves the int16 range
// stalls loudly (StallPT) and adds nothing, with one exception: a toughness
// below the range (-0/-9999) is saturated when the creature's toughness is
// still zero or less, because it is put into the graveyard at the next state-
// based action check (CR 704.5f) and the magnitude is never read again.
func (s *State) eotAdd(k *prep, sl Slot, a, b int) bool {
	op, ot := s.eotPT(sl)
	np, nt := op+a, ot+b
	if np != int(int16(np)) || nt != int(int16(nt)) {
		if np == int(int16(np)) && nt < math.MinInt16 {
			if !s.setEotPT(sl, np, math.MinInt16) {
				return false
			}
			if _, tough := s.pt(k, sl); tough <= 0 {
				return true
			}
			s.setEotPT(sl, op, ot)
		}
		s.capFull(StallPT)
		return false
	}
	return s.setEotPT(sl, np, nt)
}

// pumpAllEach is OpPumpAllEach (gorge effPump over a multi-object Defined$:
// numForObjectText per object, so Double reads that creature's own derived
// power / toughness). An amount that leaves the int16 end-of-turn range
// stalls loudly (StallPT) rather than wrapping.
func (s *State) pumpAllEach(k *prep, in *kop.Instr) {
	a, b := s.value(k, in.A), s.value(k, in.B)
	n, lo, hi := 0, 1<<30, -(1 << 30)
	fp, ft := 0, 0
	// gorge's sweep order (AliveFrom(0), each battlefield in entry order):
	// Double reads each creature's P/T after the earlier ones were pumped.
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	for _, sl := range ord[:cnt] {
		if !s.isCreature(k, sl) {
			continue
		}
		n++
		pw, tg := s.pt(k, sl)
		lo, hi = min(lo, pw), max(hi, pw)
		if DebugPumpEachFirst {
			if n == 1 {
				fp, ft = pw, tg
			}
			pw, tg = fp, ft
		}
		dp, dt := a, b
		if in.Obj&kop.PumpEachPower != 0 {
			dp = pw
		}
		if in.Obj&kop.PumpEachTough != 0 {
			dt = tg
		}
		op, ot := s.eotPT(sl)
		np, nt := op+dp, ot+dt
		if np != int(int16(np)) || nt != int(int16(nt)) {
			s.capFull(StallPT)
			return
		}
		s.touchSl(sl)
		if !s.setEotPT(sl, np, nt) {
			return
		}
		s.grantEOT(sl, kop.KwMask(uint16(in.C)))
		s.promoKw(k, sl, kop.KwMask(uint16(in.C)))
	}
	switch {
	case n == 0:
		kdHit(KdPumpEmpty)
	case n >= 2 && in.Obj != 0:
		kdHit(KdPumpEach)
		if lo != hi {
			kdHit(KdPumpEachDiff)
		}
	case n >= 2:
		kdHit(KdPumpSweep)
	}
}

// doubleCountersAll is OpDoubleCountersAll: the OpDoubleCounters body on
// each matching permanent in gorge's sweep order (sweepHits).
func (s *State) doubleCountersAll(k *prep, in *kop.Instr) {
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	for _, sl := range ord[:n] {
		s.doubleCountersAt(k, sl, in.C)
	}
	switch {
	case n == 0:
		kdHit(KdSweepEmpty)
	case n >= 2:
		kdHit(KdDoubleSweep)
	}
}

// doubleCountersAt is OpDoubleCounters on the permanent at sl: each counter
// kind c (every kind for 255) gets its own count added, through the counter
// replacements, and CounterAddedOnce fires once with the total added.
func (s *State) doubleCountersAt(k *prep, sl Slot, ck int16) {
	s.touchSl(sl)
	p := &s.perms[sl]
	added := 0
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		if !kop.IsCounterKind(c) {
			continue // CtrLevel is no counter (kw_class.go)
		}
		if ck != 255 && c != int(ck) {
			continue
		}
		n := p.ctrN(s, c)
		if n > 0 {
			before := n
			s.placeCounters(k, sl, c, n, p.ctrl, ctrPutEffect) // gorge MultiplyCounter adds n, itself replaceable
			added += p.ctrN(s, c) - before
			if p.ctrN(s, c) > before && k.anyCtrAdded {
				s.ctrNote(k, sl, c)
			}
		}
	}
	if added > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		if ck != 255 {
			// the CounterAddedOnce kind filter (triggers_gen.go) reads lastCK:
			// a stale kind from an earlier counter step (a Saga's LORE) made
			// a doubled +1/+1 counter fire the Saga's chapter trigger. The
			// every-kind form still fires once with the total (unchanged).
			s.g.lastCK = kop.CounterKind(ck)
		}
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(added))
	}
}

// kdefPutHit counts a Defined$ Valid put sweep over cnt permanents.
//
//go:noinline
func (s *State) kdefPutHit(cnt int) {
	switch {
	case cnt == 0:
		kdHit(KdSweepEmpty)
	case cnt >= 2:
		kdHit(KdPutSweep)
	}
}
