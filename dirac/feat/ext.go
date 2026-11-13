package feat

import (
	"math/bits"
	"strconv"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The value head's extension block: State.Ext lists (slot, value) pairs over
// ExtWidth fixed slots, each computed arithmetically (nothing hashed). It is
// off by default (SetExt(0)): Ext is then empty and every other output is
// unchanged. The economy (E), turn-structure (T), stack-target (S,
// ext_stack.go), impact (I) and belief (B) groups are built; SetExt accepts
// the kill group and it writes nothing yet.

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
	xITopMe     = 400 // I: impact 400..441
	xITopOpp    = 404
	xIWorthy    = 408
	xILowOnly   = 409
	xITopRemOpp = 410 // 410..416, 417..423
	xITopRemMe  = 417
	xICtxStr    = 424 // 424..427, 428..432, 433..437
	xICtxHand   = 428
	xICtxOpen   = 433
	xIProtect   = 438
	xIBestHex   = 439
	xIBestIndes = 440
	xICounter   = 441
	xIEnd       = 442
	// K: kill conditions and clocks.
	xKClockMe       = 448 // me 448..456, opp 457..465
	xKClockOpp      = 457
	xKEvClockMe     = 466 // me 466..474, opp 475..483
	xKEvClockOpp    = 475
	xKReach         = 484
	xKBurnReach     = 485
	xKStackOppDmgMe = 486
	xKStackMeDmgOpp = 487
	xKStackOppKills = 488
	xKStackMeKills  = 489
	xKCounterMe     = 490
	xKCounterOpp    = 491
	xKMeLethalStack = 492
	xKOppLethalCast = 493
	xKEnd           = 494
	xThreat         = 496 // B: unseen strong cards 496..500, unseen answers 501..505
	xAnswer         = 501
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

// entryTarget scans an entry's targets once: the first creature it aims at,
// the first player class, the first stack entry index, and the number of
// distinct real targets. Classes are targetClass's.
func (e *Encoder) entryTarget(seat uint8, se *dirac.ProjStack) (cre *vcard, player, stack, n int) {
	player, stack = tcNone, -1
	var seen [kop.MaxTargets + len(se.Extra)]uint8
	nseen := 0
	total := len(se.Targets) + int(se.NExtra)
	for j := 0; j < total; j++ {
		t := se.Extra[max(j-len(se.Targets), 0)]
		if j < len(se.Targets) {
			t = se.Targets[j]
		}
		if dirac.TCode(t) == dirac.NoTarget {
			continue
		}
		dup := false
		for k := 0; k < nseen; k++ {
			if seen[k] == t {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		seen[nseen] = t
		nseen++
		n++
		tc, host := e.targetClass(seat, t)
		switch {
		case host != nil && (tc == tcCreatureMe || tc == tcCreatureOpp):
			if cre == nil {
				cre = host
			}
		case tc == tcPlayerMe || tc == tcPlayerOpp:
			if player == tcNone {
				player = tc
			}
		case tc == tcStack:
			if stack < 0 {
				stack = e.stackEntryOf(t)
			}
		}
	}
	return
}

// isDraw reports whether a card's spell API is Draw; it compares the bytes
// without converting them to a string.
func isDraw(api []byte) bool {
	return len(api) == 4 && api[0] == 'D' && api[1] == 'r' && api[2] == 'a' && api[3] == 'w'
}

// imp is the impact of spell entry i if it resolves, in live-strength
// units before bucketing: the value the head spends a removal or a counter
// on. An ability entry is 0.
func (e *Encoder) imp(seat uint8, i int, me, opp *vplayer) int32 {
	if i < 0 || i >= len(e.v.stack) || i >= int(e.kp.NStack) || e.kp.Stack[i].Kind != 0 {
		return 0
	}
	st := &e.v.stack[i]
	if st.cv < 0 {
		return 0
	}
	c := &e.v.cards[st.cv]
	ef := &c.c.eff
	cre, player, stack, n := e.entryTarget(seat, &e.kp.Stack[i])
	smv := stackMV(c)
	switch ef.Class {
	case EffKill, EffDamage, EffDebuff:
		if cre != nil {
			if resolvesKills(ef, int32(c.rel.x), cre) {
				return liveStrength(cre)
			}
			return liveStrength(cre) / 2
		}
		if ef.Class == EffDamage && player != tcNone {
			amt := int32(ef.Amount)
			if ef.Amount == -1 {
				amt = int32(c.rel.x)
			} else if ef.Amount < 0 {
				return 2 * smv
			}
			life := me.life
			if player == tcPlayerOpp {
				life = opp.life
			}
			if amt >= life {
				return max(2*amt, 13) // lethal: strength bucket 3
			}
			return 2 * amt
		}
	case EffCounter:
		if stack >= 0 && e.v.stack[stack].cv >= 0 {
			return stackPstr(&e.v.cards[e.v.stack[stack].cv])
		}
	case EffBounce:
		if cre != nil {
			return liveStrength(cre)
		}
	case EffPump:
		if cre != nil {
			return int32(ef.NumAtt) + int32(ef.NumDef) + liveStrength(cre)/2
		}
	case EffOther:
		if isDraw(c.c.api) {
			amt := int32(ef.Amount)
			if ef.Amount == -1 {
				amt = int32(c.rel.x)
			} else if ef.Amount < 0 {
				return 2 * smv
			}
			return 3 * amt
		}
	}
	if n == 0 && c.c.fold&(tbInstant|tbSorcery) == 0 {
		return c.c.pstr // a permanent spell with no target
	}
	return 2 * smv
}

// extImpact writes group I for seat: the impact of each side's topmost spell,
// the worth of my hand removal, the tempo of a removal on the stack, and the
// counter-decision and protection context.
func (e *Encoder) extImpact(seat uint8, me, opp *vplayer) {
	v := &e.v
	n := int(e.kp.NStack)
	if n > len(v.stack) {
		n = len(v.stack)
	}
	var top [2]bool
	for i := n - 1; i >= 0 && (!top[0] || !top[1]); i-- {
		se := &e.kp.Stack[i]
		if se.Kind != 0 {
			continue
		}
		s := 0
		if se.Ctrl != seat {
			s = 1
		}
		if top[s] {
			continue
		}
		top[s] = true
		e.extOneHot(xITopMe+4*s, int32(StrengthBucket(e.imp(seat, i, me, opp))), 3)
	}
	// 408/409: does my castable removal have a worthy target?
	hasRemoval, hasCreature, worthy := false, false, false
	for _, ci := range me.hand {
		c := &v.cards[ci]
		if c.c.land || c.c.mv > me.avail {
			continue
		}
		ef := &c.c.eff
		if !(ef.Class == EffKill || ef.Class == EffDamage || ef.Class == EffDebuff) || !ef.TgtCreat {
			continue
		}
		hasRemoval = true
		for _, oi := range opp.bf {
			t := &v.cards[oi]
			if !t.creature() {
				continue
			}
			hasCreature = true
			if t.c.mv >= c.c.mv || liveStrength(t) >= 2*c.c.mv+2 {
				worthy = true
			}
		}
	}
	if hasRemoval && hasCreature {
		if worthy {
			e.extAdd(xIWorthy, 1)
		} else {
			e.extAdd(xILowOnly, 1)
		}
	}
	// 410-423: the topmost removal each side has aimed at a creature, as the
	// target's mana value against the spell's stack mana value.
	var remTop [2]bool
	for i := n - 1; i >= 0 && (!remTop[0] || !remTop[1]); i-- {
		se := &e.kp.Stack[i]
		if se.Kind != 0 || v.stack[i].cv < 0 {
			continue
		}
		s := 0
		if se.Ctrl != seat {
			s = 1
		}
		if remTop[s] {
			continue
		}
		c := &v.cards[v.stack[i].cv]
		ef := &c.c.eff
		if !(ef.Class == EffKill || ef.Class == EffDamage || ef.Class == EffDebuff) {
			continue
		}
		cre, _, _, _ := e.entryTarget(seat, se)
		if cre == nil {
			continue
		}
		remTop[s] = true
		k := int(max(-3, min(3, cre.c.mv-stackMV(c)))) + 3
		base := xITopRemMe
		if s == 1 {
			base = xITopRemOpp
		}
		e.extAdd(base+k, 1)
	}
	// 424-437: the counter decision's context: the spell I could counter,
	// and the opponent's hand and open mana.
	counterCastable := false
	protect := false
	for _, ci := range me.hand {
		c := &v.cards[ci]
		if c.c.land || c.c.mv > me.avail {
			continue
		}
		ef := &c.c.eff
		if ef.Class == EffCounter {
			counterCastable = true
		}
		if ef.Class == EffPump && (c.c.instantType || c.hasFlash()) {
			protect = true
		}
	}
	if counterCastable {
		topOpp := -1
		for i := n - 1; i >= 0; i-- {
			if e.kp.Stack[i].Kind == 0 && e.kp.Stack[i].Ctrl != seat {
				topOpp = i
				break
			}
		}
		if topOpp >= 0 && v.stack[topOpp].cv >= 0 {
			e.extOneHot(xICtxStr, int32(StrengthBucket(stackPstr(&v.cards[v.stack[topOpp].cv]))), 3)
			e.extOneHot(xICtxHand, opp.handN, 4)
			e.extOneHot(xICtxOpen, opp.avail, 4)
		}
	}
	if protect {
		e.extAdd(xIProtect, 1)
	}
	// 439/440: my max-str creature's protection keywords.
	best := int32(-1)
	for _, ci := range me.bf {
		if c := &v.cards[ci]; c.creature() {
			best = max(best, liveStrength(c))
		}
	}
	if best >= 0 {
		for _, ci := range me.bf {
			c := &v.cards[ci]
			if !c.creature() || liveStrength(c) != best {
				continue
			}
			if c.hasKw(kop.KwHexproof) || c.hasKw(kop.KwWard) {
				e.extAdd(xIBestHex, 1)
			}
			if c.hasKw(kop.KwIndestructible) {
				e.extAdd(xIBestIndes, 1)
			}
		}
	}
	if counterCastable {
		e.extAdd(xICounter, 1)
	}
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
	if e.ext&ExtImpact != 0 {
		e.extImpact(seat, me, opp)
	}
	if e.ext&ExtKill != 0 {
		// every encode path runs mzGlobal first, which stashes the aggregates
		mine, theirs := e.killMine, e.killTheirs
		e.extKill(seat, me, opp, &mine, &theirs)
		if theirs.canAttackPower >= me.life && nCastI > 0 {
			e.extAdd(xKOppLethalCast, 1)
		}
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
	case n < xITopOpp:
		return one(xITopMe, "I|top|me|imp=")
	case n < xIWorthy:
		return one(xITopOpp, "I|top|opp|imp=")
	case n == xIWorthy:
		return "I|hand-removal|worthy"
	case n == xILowOnly:
		return "I|hand-removal|low-only"
	case n < xITopRemMe:
		return one(xITopRemOpp, "I|top-removal|opp|tempo=")
	case n < xICtxStr:
		return one(xITopRemMe, "I|top-removal|me|tempo=")
	case n < xICtxHand:
		return one(xICtxStr, "I|counterctx|str=")
	case n < xICtxOpen:
		return one(xICtxHand, "I|counterctx|opp-hand=")
	case n < xIProtect:
		return one(xICtxOpen, "I|counterctx|opp-open=")
	case n == xIProtect:
		return "I|me|protect-castable"
	case n == xIBestHex:
		return "I|my-best|hexproof-or-ward"
	case n == xIBestIndes:
		return "I|my-best|indestructible"
	case n == xICounter:
		return "I|me|counter-castable"
	case n < 448:
		return "reserved"
	case n < xKEnd:
		return extKillSlotName(n)
	case n < 496:
		return "reserved"
	case n < xAnswer:
		return one(xThreat, "B|opp-unseen-threats=")
	case n < 506:
		return one(xAnswer, "B|opp-unseen-answers=")
	}
	return "reserved"
}

// Group K (ExtKill), slots 448-493: the two mz lethal rows
// (mzGlobal, state.go) crossed with the turns-to-kill clock, the side's
// burn reach and the stack's lethal reads. The mz rows themselves are not
// touched. Live strength is not used here: the clock is raw power, because
// the row is a turns count, not a value.
//
// Approximations: burnHand reads only the seat's own hand (never the
// opponent's), takes castable EffDamage cards in ascending MV while their
// mana fits, and reads an X amount as the leftover mana; an unknown amount
// (-2) counts 0. A stack entry's damage is the first spell ability's
// EffectSpec Amount (X as its committed rel.x), and a "removes the last
// blocker" entry is a Kind-0 kill/bounce targeting the defender's only
// untapped creature.

// clockP is a side's next-turn attacking power: every non-defender
// creature's positive power, tapped and summoning-sick included.
func (e *Encoder) clockP(pv *vplayer) int32 {
	var p int32
	for _, ci := range pv.bf {
		c := &e.v.cards[ci]
		if c.creature() && !c.hasDefender() && c.power > 0 {
			p += c.power
		}
	}
	return p
}

// flyingP is clockP's evasive twin: the side's flying creatures' power.
func (e *Encoder) flyingP(pv *vplayer) int32 {
	var p int32
	for _, ci := range pv.bf {
		c := &e.v.cards[ci]
		if c.creature() && !c.hasDefender() && c.power > 0 && c.hasKw(kop.KwFlying) {
			p += c.power
		}
	}
	return p
}

// hasFlyBlock reports whether pv controls a flying or reach creature, the
// blockers that turn the evasive clock off.
func (e *Encoder) hasFlyBlock(pv *vplayer) bool {
	for _, ci := range pv.bf {
		c := &e.v.cards[ci]
		if c.creature() && (c.hasKw(kop.KwFlying) || c.hasKw(kop.KwReach)) {
			return true
		}
	}
	return false
}

// clockT is ceil(life/P) clamped to 1..8, 8 when P is 0 (no clock).
func clockT(life, p int32) int32 {
	if p <= 0 {
		return 8
	}
	t := int32(1)
	if life > 0 {
		t = (life + p - 1) / p
	}
	if t < 1 {
		t = 1
	}
	if t > 8 {
		t = 8
	}
	return t
}

// burnHand is the seat's hand burn reach: castable (nonland, mv <= avail)
// EffDamage cards that can target a player, in ascending MV while their
// mana fits. X spends the leftover mana; an unknown amount counts 0.
func (e *Encoder) burnHand(me *vplayer) int32 {
	var idx [dirac.MaxObj]int32
	n := 0
	for _, ci := range me.hand {
		c := &e.v.cards[ci]
		ef := &c.c.eff
		if c.c.land || c.c.mv > me.avail || ef.Class != EffDamage || !ef.TgtPlayer {
			continue
		}
		j := n
		for j > 0 && e.v.cards[idx[j-1]].c.mv > c.c.mv {
			idx[j] = idx[j-1]
			j--
		}
		idx[j] = ci
		n++
	}
	var sum, dmg int32
	for k := 0; k < n; k++ {
		c := &e.v.cards[idx[k]]
		if sum+c.c.mv > me.avail {
			break
		}
		sum += c.c.mv
		amt := int32(c.c.eff.Amount)
		if amt == -1 { // X: the leftover mana after this card's MV
			dmg += max(me.avail-sum, 0)
			break
		}
		if amt > 0 {
			dmg += amt
		}
	}
	return dmg
}

// entrySpell is stack entry i's spell card and effect, false for an ability
// entry or a resolved one.
func (e *Encoder) entrySpell(i int) (*vcard, *EffectSpec, bool) {
	if i < 0 || i >= int(e.kp.NStack) || i >= len(e.v.stack) {
		return nil, nil, false
	}
	if e.kp.Stack[i].Kind != 0 || e.v.stack[i].cv < 0 {
		return nil, nil, false
	}
	c := &e.v.cards[e.v.stack[i].cv]
	return c, &c.c.eff, true
}

// knownAmount is a stack spell's damage: the integer amount, X as its
// committed rel.x, and an unknown amount as 0.
func knownAmount(ef *EffectSpec, x int32) int32 {
	switch {
	case ef.Amount > 0:
		return int32(ef.Amount)
	case ef.Amount == -1 && x > 0:
		return x
	}
	return 0
}

// hitsPlayer reports whether stack entry i targets a player of class tc.
func (e *Encoder) hitsPlayer(seat uint8, i, tc int) bool {
	se := &e.kp.Stack[i]
	for j := 0; j < len(se.Targets)+int(se.NExtra); j++ {
		t := se.Extra[max(j-len(se.Targets), 0)]
		if j < len(se.Targets) {
			t = se.Targets[j]
		}
		if dirac.TCode(t) != dirac.NoTarget {
			if tc2, _ := e.targetClass(seat, t); tc2 == tc {
				return true
			}
		}
	}
	return false
}

// hitsObj reports whether stack entry i targets battlefield creature obj,
// whose controller's class (tcCreatureMe or tcCreatureOpp) is want.
func (e *Encoder) hitsObj(seat uint8, i int, obj uint8, want int) bool {
	se := &e.kp.Stack[i]
	for j := 0; j < len(se.Targets)+int(se.NExtra); j++ {
		t := se.Extra[max(j-len(se.Targets), 0)]
		if j < len(se.Targets) {
			t = se.Targets[j]
		}
		if dirac.TCode(t) != dirac.NoTarget && t == obj {
			if tc, host := e.targetClass(seat, t); tc == want && host != nil {
				return true
			}
		}
	}
	return false
}

// onlyUntapped is pv's only untapped creature's object, false unless
// exactly one exists.
func (e *Encoder) onlyUntapped(pv *vplayer) (uint8, bool) {
	var o uint8
	n := 0
	for _, ci := range pv.bf {
		c := &e.v.cards[ci]
		if c.creature() && !c.tapped {
			o, n = c.obj, n+1
		}
	}
	return o, n == 1
}

// extKill writes group K for seat.
func (e *Encoder) extKill(seat uint8, me, opp *vplayer, mine, theirs *agg) {
	e.extOneHot(xKClockMe, clockT(opp.life, e.clockP(me)), 8)
	e.extOneHot(xKClockOpp, clockT(me.life, e.clockP(opp)), 8)
	var evMe, evOpp int32
	if !e.hasFlyBlock(opp) {
		evMe = e.flyingP(me)
	}
	if !e.hasFlyBlock(me) {
		evOpp = e.flyingP(opp)
	}
	e.extOneHot(xKEvClockMe, clockT(opp.life, evMe), 8)
	e.extOneHot(xKEvClockOpp, clockT(me.life, evOpp), 8)

	burn := e.burnHand(me)
	if mine.canAttackPower+burn >= opp.life {
		e.extAdd(xKReach, 1)
	}
	onlyMe, oneMe := e.onlyUntapped(me)
	onlyOpp, oneOpp := e.onlyUntapped(opp)
	var dmgAtMe, dmgAtOpp int32
	var oppKills, meKills, oppSpell bool
	for i := 0; i < int(e.kp.NStack); i++ {
		se := &e.kp.Stack[i]
		if se.Kind != 0 {
			continue
		}
		if se.Ctrl != seat {
			oppSpell = true
		}
		c, ef, ok := e.entrySpell(i)
		if !ok {
			continue
		}
		switch {
		case ef.Class == EffDamage && se.Ctrl != seat:
			if e.hitsPlayer(seat, i, tcPlayerMe) {
				dmgAtMe += knownAmount(ef, int32(c.rel.x))
			}
		case ef.Class == EffDamage && se.Ctrl == seat:
			if e.hitsPlayer(seat, i, tcPlayerOpp) {
				dmgAtOpp += knownAmount(ef, int32(c.rel.x))
			}
		}
		if ef.Class == EffKill || ef.Class == EffBounce {
			if se.Ctrl != seat && oneMe && e.hitsObj(seat, i, onlyMe, tcCreatureMe) {
				oppKills = true
			}
			if se.Ctrl == seat && oneOpp && e.hitsObj(seat, i, onlyOpp, tcCreatureOpp) {
				meKills = true
			}
		}
	}
	set486 := dmgAtMe >= me.life
	set487 := dmgAtOpp >= opp.life
	set488 := oppKills && e.clockP(opp) >= me.life
	set489 := meKills && e.clockP(me) >= opp.life
	if set486 {
		e.extAdd(xKStackOppDmgMe, 1)
	}
	if set487 {
		e.extAdd(xKStackMeDmgOpp, 1)
	}
	if set488 {
		e.extAdd(xKStackOppKills, 1)
	}
	if set489 {
		e.extAdd(xKStackMeKills, 1)
	}
	if burn+dmgAtOpp >= opp.life {
		e.extAdd(xKBurnReach, 1)
	}
	// Counters: my counter on an entry that set 486/488, the mirror for
	// 487/489.
	if set486 || set487 || set488 || set489 {
		ctrMe, ctrOpp := false, false
		for i := 0; i < int(e.kp.NStack) && !(ctrMe && ctrOpp); i++ {
			se := &e.kp.Stack[i]
			if se.Kind != 0 {
				continue
			}
			_, ef, ok := e.entrySpell(i)
			if !ok || ef.Class != EffCounter {
				continue
			}
			for j := 0; j < len(se.Targets)+int(se.NExtra); j++ {
				t := se.Extra[max(j-len(se.Targets), 0)]
				if j < len(se.Targets) {
					t = se.Targets[j]
				}
				k := e.stackEntryOf(t)
				if k < 0 {
					continue
				}
				kc, kef, kok := e.entrySpell(k)
				if !kok {
					continue
				}
				ks := &e.kp.Stack[k]
				// the counter's controller picks the side: mine on their
				// entry (490), theirs on mine (491)
				if se.Ctrl == seat && ks.Ctrl != seat {
					if set486 && kef.Class == EffDamage && e.hitsPlayer(seat, k, tcPlayerMe) && knownAmount(kef, int32(kc.rel.x)) > 0 {
						ctrMe = true
					}
					if set488 && oneMe && (kef.Class == EffKill || kef.Class == EffBounce) && e.hitsObj(seat, k, onlyMe, tcCreatureMe) {
						ctrMe = true
					}
				} else if se.Ctrl != seat && ks.Ctrl == seat {
					if set487 && kef.Class == EffDamage && e.hitsPlayer(seat, k, tcPlayerOpp) && knownAmount(kef, int32(kc.rel.x)) > 0 {
						ctrOpp = true
					}
					if set489 && oneOpp && (kef.Class == EffKill || kef.Class == EffBounce) && e.hitsObj(seat, k, onlyOpp, tcCreatureOpp) {
						ctrOpp = true
					}
				}
			}
		}
		if ctrMe {
			e.extAdd(xKCounterMe, 1)
		}
		if ctrOpp {
			e.extAdd(xKCounterOpp, 1)
		}
	}
	if mine.canAttackPower >= opp.life && oppSpell {
		e.extAdd(xKMeLethalStack, 1)
	}
}

// extKillSlotName names a K slot (448..493).
func extKillSlotName(n int) string {
	one := func(base int, prefix string) string { return prefix + strconv.Itoa(n-base) }
	switch {
	case n < xKClockOpp:
		return one(xKClockMe, "K|clock|me=")
	case n < xKEvClockMe:
		return one(xKClockOpp, "K|clock|opp=")
	case n < xKEvClockOpp:
		return one(xKEvClockMe, "K|evclock|me=")
	case n < xKReach:
		return one(xKEvClockOpp, "K|evclock|opp=")
	}
	return [...]string{
		"K|me|lethal-reach",
		"K|me|burn-reach",
		"K|stack|opp-dmg-lethal-me",
		"K|stack|me-dmg-lethal-opp",
		"K|stack|opp-removes-last-blocker",
		"K|stack|me-removes-last-blocker",
		"K|stack|my-counter-prevents-lethal",
		"K|stack|opp-counter-prevents-my-lethal",
		"K|lethal-unblocked×opp-on-stack",
		"K|opp-lethal-unblocked×my-castable-i",
	}[n-xKReach]
}
