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
		if t == dirac.NoTarget {
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
		if string(c.c.api) == "Draw" {
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
