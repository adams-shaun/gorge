package feat

import (
	"math/bits"
	"strconv"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The value head's extension block: State.Ext lists (slot, value) pairs over
// ExtWidth fixed slots, each computed arithmetically (nothing hashed). It is
// off by default (SetExt(0)): Ext is then empty and every other output is
// unchanged. The economy (E), turn-structure (T), stack-target (S,
// ext_stack.go) and belief (B) groups are built; SetExt accepts the other
// groups and they write nothing yet.

// ExtWidth is the block's slot count; ExtLayoutVersion names its layout.
const (
	ExtWidth         = 512
	ExtLayoutVersion = 1
)

// ExtGroups selects the slot groups an encode writes.
type ExtGroups uint8

// The slot groups.
const (
	ExtEconomy ExtGroups = 1 << iota
	ExtTurn
	ExtStackTarget
	ExtImpact
	ExtKill
	ExtBelief
)

// ExtAll is every group.
const ExtAll = ExtEconomy | ExtTurn | ExtStackTarget | ExtImpact | ExtKill | ExtBelief

// Slot bases.
const (
	xBfmv       = 0
	xBfmvDiff   = 2
	xLandsMe    = 15
	xLandsOpp   = 26
	xOppHandN   = 37
	xOppHand    = 45
	xOpenMine   = 46
	xOpenOpp    = 55
	xStackTopMe = 64
	xStackTopOp = 73
	xStackMV    = 82
	xCastI      = 84 // counter, removal, trick, other
	xEEnd       = 96
	xMains      = 96 // me 96..98, opp 99..101
	xPredraw    = 102
	xPremature  = 104
	xTEnd       = 112
	xThreat     = 496 // B: unseen strong cards 496..500, unseen answers 501..505
	xAnswer     = 501
)

// SetOppBelief sets the believed opponent list, counts indexed by kop.CardID
// (the slice is kept, not copied, and must stay unchanged until cleared); nil
// clears it. Group B reads it only while ExtBelief is on.
//
// Pass the search's closed-list belief (the hosted player's), not the
// true list: a net trained on the true list learns a fact hosted play never has.
func (e *Encoder) SetOppBelief(counts []uint8) {
	e.belief, e.bThreat, e.bAnswer = counts, 0, 0
	if counts == nil {
		e.bseen, e.bkind = nil, nil
		return
	}
	e.bseen = make([]uint8, len(counts))
	e.bkind = make([]uint8, len(counts))
	for i, n := range counts {
		if n == 0 {
			continue
		}
		c := e.pool.card(kop.CardID(i))
		ef := &c.eff
		if StrengthBucket(c.pstr) >= 2 {
			e.bkind[i] |= 1
			e.bThreat += int32(n)
		}
		if ef.Class == EffCounter || (ef.Class == EffKill || ef.Class == EffDamage || ef.Class == EffDebuff) && ef.TgtCreat {
			e.bkind[i] |= 2
			e.bAnswer += int32(n)
		}
	}
}

// extBelief writes group B: the believed opponent cards not yet seen in the
// opponent's public zones or on the stack under its control.
func (e *Encoder) extBelief(seat uint8, opp *vplayer) {
	v := &e.v
	threat, answer := e.bThreat, e.bAnswer
	see := func(cid kop.CardID) {
		i := int(cid)
		if i >= len(e.belief) {
			return
		}
		e.bseen[i]++
		if e.bseen[i] <= e.belief[i] {
			if e.bkind[i]&1 != 0 {
				threat--
			}
			if e.bkind[i]&2 != 0 {
				answer--
			}
		}
	}
	for _, l := range [3][]int32{opp.bf, opp.grave, opp.exile} {
		for _, ci := range l {
			see(v.cards[ci].cid)
		}
	}
	for i := range v.stack {
		st := &v.stack[i]
		if e.kp.Stack[i].Kind == 0 && st.cv >= 0 && st.ctrl != seat {
			see(v.cards[st.cv].cid)
		}
	}
	for _, l := range [3][]int32{opp.bf, opp.grave, opp.exile} {
		for _, ci := range l {
			if i := int(v.cards[ci].cid); i < len(e.bseen) {
				e.bseen[i] = 0
			}
		}
	}
	for i := range v.stack {
		if st := &v.stack[i]; st.cv >= 0 && st.ctrl != seat {
			if j := int(v.cards[st.cv].cid); j < len(e.bseen) {
				e.bseen[j] = 0
			}
		}
	}
	e.extOneHot(xThreat, threat, 4)
	e.extOneHot(xAnswer, answer, 4)
}

// SetExt selects the extension groups; 0 (the default) turns the block off.
func (e *Encoder) SetExt(g ExtGroups) { e.ext = g }

// ExtGroups is the selected groups.
func (e *Encoder) ExtGroups() ExtGroups { return e.ext }

// ExtHash names the layout of groups g: FNV-1a of the version, width and groups.
func ExtHash(g ExtGroups) uint64 {
	h := add(off, "gorge-feat-ext\x1fv=")
	h = addInt(h, ExtLayoutVersion)
	h = add(h, "\x1fwidth=")
	h = addInt(h, ExtWidth)
	h = add(h, "\x1fgroups=")
	return addInt(h, int64(g))
}

func (e *Encoder) extAdd(slot int, v float32) {
	e.extAcc[slot] += v
	e.extHit[slot>>6] |= 1 << (slot & 63)
}

// extOneHot adds 1 at base+clamp(n, 0, hi).
func (e *Encoder) extOneHot(base int, n, hi int32) { e.extAdd(base+int(clampI(n, hi)), 1) }

// extFor is the hooks' entry: the block for the encode just built, an empty
// view of the encoder's storage when off.
func (e *Encoder) extFor(seat uint8, me, opp *vplayer) []Feature {
	if e.ext == 0 {
		return e.extOut[:0]
	}
	return e.encodeExt(seat, me, opp)
}

// stackMV is a spell entry's mana value on the stack: mv + X for an X cost.
func stackMV(c *vcard) int32 {
	mv := c.c.mv
	if c.c.eff.CostX {
		mv += int32(c.rel.x)
	}
	return mv
}

func (e *Encoder) encodeExt(seat uint8, me, opp *vplayer) []Feature {
	v := &e.v
	var castI [4]int32
	// castable instant-speed hand counts (read by E slots 84-87 and the
	// premature window)
	for _, ci := range me.hand {
		c := &v.cards[ci]
		if c.c.land || c.c.mv > me.avail || !(c.c.instantType || c.hasFlash()) {
			continue
		}
		ef := &c.c.eff
		switch {
		case ef.Class == EffCounter:
			castI[0]++
		case (ef.Class == EffKill || ef.Class == EffDamage || ef.Class == EffDebuff) && ef.TgtCreat:
			castI[1]++
		case ef.Class == EffPump:
			castI[2]++
		default:
			castI[3]++
		}
	}
	nCastI := castI[0] + castI[1] + castI[2] + castI[3]
	if e.ext&ExtEconomy != 0 {
		var bfmv, lands [2]int32
		for s, pv := range [2]*vplayer{me, opp} {
			for _, ci := range pv.bf {
				c := &v.cards[ci]
				if c.land() {
					lands[s]++
				} else {
					bfmv[s] += c.c.mv
				}
			}
		}
		for s := 0; s < 2; s++ {
			if bfmv[s] > 0 {
				e.extAdd(xBfmv+s, float32(bfmv[s])/10)
			}
		}
		d := bfmv[0] - bfmv[1]
		e.extOneHot(xBfmvDiff, floorDiv2(d)+6, 12)
		e.extOneHot(xLandsMe, lands[0], 10)
		e.extOneHot(xLandsOpp, lands[1], 10)
		e.extOneHot(xOppHandN, opp.handN, 7)
		e.extAdd(xOppHand, float32(clampI(opp.handN, 7))/7)
		if v.active != seat {
			e.extOneHot(xOpenMine, me.avail, 8)
		} else {
			e.extOneHot(xOpenOpp, opp.avail, 8)
		}
		var sum [2]int32
		var has, top [2]bool
		for i := len(v.stack) - 1; i >= 0; i-- {
			st := &v.stack[i]
			if e.kp.Stack[i].Kind != 0 || st.cv < 0 {
				continue
			}
			s := 0
			if st.ctrl != seat {
				s = 1
			}
			mv := stackMV(&v.cards[st.cv])
			has[s] = true
			sum[s] += mv
			if !top[s] {
				top[s] = true
				e.extOneHot(xStackTopMe+9*s, mv, 8)
			}
		}
		for s := 0; s < 2; s++ {
			if has[s] {
				e.extAdd(xStackMV+s, float32(sum[s])/10)
			}
		}
		for k, n := range castI {
			if n > 0 {
				e.extAdd(xCastI+k, float32(n))
			}
		}
	}
	if e.ext&ExtTurn != 0 && v.step >= 0 {
		k := 0
		switch {
		case v.step <= 3:
			k = 2
		case v.step <= 9:
			k = 1
		}
		mine := v.active == seat
		if mine {
			e.extAdd(xMains+k, 1)
		} else {
			e.extAdd(xMains+3+k, 1)
		}
		predraw := (v.step == 0 || v.step == 1) && v.turn > 1
		if predraw {
			if mine {
				e.extAdd(xPredraw, 1)
			} else {
				e.extAdd(xPredraw+1, 1)
			}
		}
		if mine && (predraw || v.step == 2) && len(v.stack) == 0 && nCastI > 0 {
			e.extAdd(xPremature, 1)
		}
	}
	if e.ext&ExtStackTarget != 0 {
		e.extStack(seat, me, opp)
	}
	if e.ext&ExtBelief != 0 && e.belief != nil {
		e.extBelief(seat, opp)
	}
	out := e.extOut[:0]
	for w := range e.extHit {
		for b := e.extHit[w]; b != 0; b &= b - 1 {
			slot := w<<6 | bits.TrailingZeros64(b)
			out = append(out, Feature{Row: uint16(slot), Value: e.extAcc[slot]})
			e.extAcc[slot] = 0
		}
		e.extHit[w] = 0
	}
	e.extOut = out
	return out
}

// floorDiv2 is floor(d/2).
func floorDiv2(d int32) int32 { return d >> 1 }

// ExtSlotName is a slot's frozen name (tests and debug; it allocates).
func ExtSlotName(slot uint16) string {
	n := int(slot)
	one := func(base int, prefix string) string { return prefix + strconv.Itoa(n-base) }
	switch {
	case n >= ExtWidth:
		return "reserved"
	case n < xBfmvDiff:
		return [...]string{"E|bfmv|me", "E|bfmv|opp"}[n]
	case n < xLandsMe:
		return one(xBfmvDiff, "E|bfmv-diff=")
	case n < xLandsOpp:
		return one(xLandsMe, "E|lands|me=")
	case n < xOppHandN:
		return one(xLandsOpp, "E|lands|opp=")
	case n < xOppHand:
		return one(xOppHandN, "E|opp-hand=")
	case n == xOppHand:
		return "E|opp-hand"
	case n < xOpenOpp:
		return one(xOpenMine, "E|me|open-oppturn=")
	case n < xStackTopMe:
		return one(xOpenOpp, "E|opp|open-myturn=")
	case n < xStackTopOp:
		return one(xStackTopMe, "E|stacktop|me|mv=")
	case n < xStackMV:
		return one(xStackTopOp, "E|stacktop|opp|mv=")
	case n == xStackMV:
		return "E|stackmv|me"
	case n == xStackMV+1:
		return "E|stackmv|opp"
	case n >= xCastI && n < xCastI+4:
		return "E|hand|castable-i|" + [...]string{"counter", "removal", "trick", "other"}[n-xCastI]
	case n < xEEnd:
		return "reserved"
	case n < xMains+3:
		return one(xMains, "T|me|mains-left=")
	case n < xPredraw && n >= xMains:
		return one(xMains+3, "T|opp|mains-left=")
	case n == xPredraw:
		return "T|me|predraw"
	case n == xPredraw+1:
		return "T|opp|predraw"
	case n == xPremature:
		return "T|me|premature-window"
	case n < xTEnd:
		return "reserved"
	case n < xSEnd:
		return extStackSlotName(n)
	case n < 442:
		return "todo:I"
	case n < 448:
		return "reserved"
	case n < 494:
		return "todo:K"
	case n < 496:
		return "reserved"
	case n < xAnswer:
		return one(xThreat, "B|opp-unseen-threats=")
	case n < 506:
		return one(xAnswer, "B|opp-unseen-answers=")
	}
	return "reserved"
}
