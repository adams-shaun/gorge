package feat

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Feature is one sparse entry (policynet.Feature's layout).
type Feature struct {
	Row   uint16
	Value float32
}

// Widths (policynet's pinned constants).
const (
	DenseWidth       = 92
	EntityRawWidth   = 100
	EntityRawWidthV1 = 68
	OptionDenseWidth = 28
	OptionSlotWidth  = 128
	// EntityRawWidthRelEnd is the end of the relational block (68..83); the
	// v2 block (colour, pips, category) starts above it.
	EntityRawWidthRelEnd = 84

	// The seat blocks: maxSeatBlocks relative seats of ten values from 28.
	maxSeatBlocks = 4

	// The Commander block: maxSeatBlocks relative seats of
	// commanderBlockWidth values, appended after the 68-wide pre-Commander
	// vector.
	denseCmd0           = 68
	commanderBlockWidth = 6
	denseCmdDamage0     = 0
	denseCmdCasts       = 3
	denseCmdOnField     = 4
	denseCmdLost        = 5
)

// Card is one entity card (policynet.EntityCard).
type Card struct {
	Group uint8
	Raw   [EntityRawWidth]float32
	Rows  [2]Feature
	NRows uint8
}

// State is one encoded state (policynet.State): Sparse and Cards are views
// of the Encoder's storage, valid until its next encode.
type State struct {
	Dense  [DenseWidth]float32
	Sparse []Feature
	Cards  []Card
	// Ext is the extension block (SetExt), empty when off
	Ext []Feature
}

// Encoder encodes kernel positions against one Pool. It owns every buffer
// it hands out; one per goroutine.
type Encoder struct {
	pool  *Pool
	kp    dirac.Projection
	v     vview
	objs  []dirac.Obj
	order []uint16
	zl    [6][]uint16 // hand/graveyard/exile buckets by owner
	st    State
	sp    []Feature
	sorts sortScratch
	cards []Card
	seen  [256]uint32
	gen   uint32
	opt   optScratch
	// st0 is the state the last EncodeState read; source its
	// dirac.FeatSource.
	st0    *dirac.State
	source uint8
	// inc is the incremental encoder's per-seat state (EncodeStateInc).
	inc [dirac.MaxSeats]*seatInc
	// IncStats counts EncodeStateInc's work.
	IncStats IncStats
	// objOverflow counts the builds whose position held an object past
	// noObj (ObjOverflow).
	objOverflow int
	// vb is the search base (SetBase); hbase is the build's base hashes
	// (hb, when the State carries vb's epoch)
	vb    *viewBase
	hbase *[dirac.MaxObj]uint64
	// kpg stamps the objects of kp written in build kgen (po)
	kpg  [dirac.MaxObj]uint32
	kgen uint32
	// parts is EncodeParts' per-seat cache.
	parts [dirac.MaxSeats]*partsSeat

	// ext is the extension groups (SetExt); extAcc/extHit accumulate a
	// block, extOut is its emission. killMine/killTheirs are the current
	// build's mz aggregates (group K), stashed by mzGlobal.
	ext        ExtGroups
	extAcc     [ExtWidth]float32
	extHit     [ExtWidth / 64]uint64
	extOut     []Feature
	killMine   agg
	killTheirs agg
	// belief is SetOppBelief's list; bseen/bkind its per-card scratch and
	// class bits (1 threat, 2 answer), bThreat/bAnswer the class totals
	belief           []uint8
	bseen, bkind     []uint8
	bThreat, bAnswer int32

	PartsStats PartsStats
	slot       [dirac.MaxObj]*partCache // this encode's slot per listed object
	// vc is the content-keyed card view cache (cachedCard); hb the
	// objects' content hashes at the base (SetBase), ohc the changed
	// objects' along a line, ctx the build's derived context (ctxOK once
	// read)
	vc    *vcCache
	hb    [dirac.MaxObj]uint64
	ohc   dirac.ObjHashCache
	ctx   uint64
	ctxOK bool
	// ohint is per object: the view it last had on a line of play and the
	// change stamp then (kernel State.ChangedSince), so a build reuses an
	// unchanged object's view without recomputing its content key.
	ohint [dirac.MaxObj]cardHint
	// oroot is per object: the view its epoch-root content has (an object
	// the kernel has not marked changed since SetEpoch is field for field
	// the root's, in every world cloned from it), so a world reuses it
	// without the line hint.
	oroot [dirac.MaxObj]cardHint
	// tym memoises the derived type lines and renamed views the pool did
	// not precompute (the pool is shared, so only read)
	tym tyMemo
}

// cardHint is one object's last card view on a line of play: the view (a
// pointer into the content cache, valid while it still holds key), the
// stamp it was built at, the build's derived context and zone class.
type cardHint struct {
	line  uint64
	at    uint32
	epoch uint32
	ctx   uint64
	key   uint64
	vc    *vcore
	z     uint8
}

// NewEncoder returns an encoder for pool's table.
func NewEncoder(pool *Pool) *Encoder {
	return &Encoder{pool: pool, sp: make([]Feature, 0, 512), cards: make([]Card, 0, 64)}
}

// SetPool rebinds the encoder to another pool (another matchup's table),
// keeping its buffers.
func (e *Encoder) SetPool(pool *Pool) {
	if pool == e.pool {
		return
	}
	e.pool = pool
	// every content cache is keyed by CardIDs of the old pool
	for _, ps := range e.parts {
		if ps != nil {
			for i := range ps.set {
				ps.set[i].key = [pcWays]uint64{}
			}
		}
	}
	for _, si := range e.inc {
		if si != nil {
			*si = seatInc{fresh: si.fresh[:0], sp: si.sp[:0], cards: si.cards[:0]}
		}
	}
	e.NoBase()
	if e.vc != nil {
		e.vc.clear()
	}
	e.ohint = [dirac.MaxObj]cardHint{}
	e.oroot = [dirac.MaxObj]cardHint{}
	e.tym.reset()
}

// Pool is the encoder's pool.
func (e *Encoder) Pool() *Pool { return e.pool }

// EncodeState encodes s at decision d for d's player and returns the
// state, valid until the next call. It also prepares the view
// EncodeOptions reads.
func (e *Encoder) EncodeState(s *dirac.State, d *dirac.Decision) *State {
	return e.EncodeStateFor(s, d, d.Player)
}

// EncodeStateFor encodes s at decision d from seat's view (azmcts's leaf
// value reads the searching seat's view whoever's decision is pending).
func (e *Encoder) EncodeStateFor(s *dirac.State, d *dirac.Decision, seat uint8) *State {
	seat, me, opp := e.prelude(s, d, seat)
	st := e.fullTail(seat, me, opp)
	st.Ext = e.extFor(seat, me, opp)
	return st
}

// prelude builds the view of s at d for seat and the dense vector; it
// returns the seat and its player blocks.
func (e *Encoder) prelude(s *dirac.State, d *dirac.Decision, seat uint8) (uint8, *vplayer, *vplayer) {
	e.st0 = s
	e.build(s, seat, d.Kind, d.Player)
	v := &e.v
	// gorge's priority holder at a decision that is not a priority round:
	// a turn-based action (declaring attackers or blockers, combat damage
	// division, the cleanup discard) is asked while the step change left
	// priority with the active player.
	switch {
	case d.Kind == dirac.DkPriority:
		v.priority = d.Player
	case d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers,
		d.Kind == dirac.DkChoose && d.Sub == dirac.ChCleanupDiscard:
		v.priority = v.active
	case d.Kind == dirac.DkChoose && (d.Sub == dirac.ChDivide || d.Sub == dirac.ChGenReplOrder && s.FeatReplOrderCombat()):
		// Asked at a combat damage step: the active player holds priority
		// from the step change, except after a first-strike damage step,
		// whose priority round (CR 510.4) ended with the non-active player.
		v.priority = v.active
		if s.FeatFirstStrikeCombat() && e.kp.Step == kop.StepDamage {
			v.priority = v.lastToPass(s)
		}
	}
	st := &e.st
	st.Dense = [DenseWidth]float32{}
	dn := &st.Dense
	dn[0] = float32(v.turn)
	b := float32(v.turn)
	if b > 40 {
		b = 40
	}
	dn[1] = b / 40
	if v.step >= 0 {
		dn[2+int(v.step)] = 1
	}
	if v.phase >= 0 {
		dn[14+int(v.phase)] = 1
	}
	if v.active == seat {
		dn[19] = 1
	}
	if v.priority == seat {
		dn[20] = 1
	}
	dn[21] = float32(len(v.stack))
	me, opp := v.sides()
	// The seat blocks: the viewer's, then each other seat in turn order from
	// it (policynet's relative seats); at two seats, me then the opponent.
	for blk := 0; blk < int(v.n) && blk < maxSeatBlocks; blk++ {
		pv := &v.players[v.relPlayer(blk)]
		base := 28 + blk*10
		dn[base+0] = float32(pv.life)
		dn[base+1] = float32(pv.handN)
		dn[base+2] = float32(pv.libN)
		dn[base+3] = float32(pv.graveN)
		var bf, cr, pw, th, untapped, untappedLands int
		for _, ci := range pv.bf {
			c := &v.cards[ci]
			bf++
			if c.creature() {
				cr++
				pw += int(c.power)
				th += int(c.tough)
			}
			if !c.tapped {
				untapped++
				if c.land() {
					untappedLands++
				}
			}
		}
		dn[base+4] = float32(bf)
		dn[base+5] = float32(cr)
		dn[base+6] = float32(pw)
		dn[base+7] = float32(th)
		dn[base+8] = float32(untapped)
		dn[base+9] = float32(untappedLands)
	}
	for k := 0; k < 6; k++ {
		dn[22+k] = float32(me.pool[k])
	}
	// The Commander block: one per relative seat, the order the seat blocks
	// use (block 0 the viewer, then turn order from it), each with a damage
	// column per other relative seat in turn order from its own seat, as
	// policynet writes them. A non-Commander game leaves the block zero
	// except the left-the-game bit.
	cmdGame := s.FeatCommanderGame()
	for blk := 0; blk < int(v.n) && blk < maxSeatBlocks; blk++ {
		p := v.relPlayer(blk)
		base := denseCmd0 + blk*commanderBlockWidth
		if s.FeatLost(p) {
			dn[base+denseCmdLost] = 1
		}
		if !cmdGame {
			continue
		}
		for k := 1; k < int(v.n) && k <= maxSeatBlocks-1; k++ {
			dmg := float32(s.FeatCommanderDamage(p, v.relPlayer(blk+k))) / 21
			if dmg > 1 {
				dmg = 1
			}
			dn[base+denseCmdDamage0+k-1] = dmg
		}
		casts := float32(s.FeatCommanderCast(p)) / 4
		if casts > 1 {
			casts = 1
		}
		dn[base+denseCmdCasts] = casts
		if cmd := s.FeatCommander(p); cmd != dirac.NoObj && e.po(uint8(cmd)).Zone == kop.ZoneBattlefield {
			dn[base+denseCmdOnField] = 1
		}
	}
	return seat, me, opp
}

// fullTail is the full rebuild of the sparse bag and the entity cards.
func (e *Encoder) fullTail(seat uint8, me, opp *vplayer) *State {
	v := &e.v
	st := &e.st
	// The v1 bag, in its fixed zone order.
	e.sp = e.sp[:0]
	for _, ci := range me.hand {
		e.push(v.cards[ci].c.nameTag[ztHand], 1)
	}
	for _, ci := range me.bf {
		e.battle(&v.cards[ci], ztBattleMe)
	}
	for _, ci := range opp.bf {
		e.battle(&v.cards[ci], ztBattleOpp)
	}
	for _, ci := range me.grave {
		e.push(v.cards[ci].c.nameTag[ztGraveMe], 1)
	}
	for _, ci := range opp.grave {
		e.push(v.cards[ci].c.nameTag[ztGraveOpp], 1)
	}
	for _, ci := range me.exile {
		e.push(v.cards[ci].c.nameTag[ztExileMe], 1)
	}
	for _, ci := range opp.exile {
		e.push(v.cards[ci].c.nameTag[ztExileOpp], 1)
	}
	for i := range v.stack {
		e.push(v.stack[i].name.nameTag[ztStack], 1)
	}
	// policynet sorts the v1 bag, appends the mz rows and sorts again; one
	// stable sort of the whole emission leaves the same order.
	e.mz(seat, me, opp)
	stableSortByRow(e.sp, &e.sorts)
	st.Sparse = e.sp
	e.entities(seat, me, opp)
	st.Cards = e.cards
	return st
}

func (e *Encoder) push(r uint16, v float32) { e.sp = append(e.sp, Feature{Row: r, Value: v}) }
func (e *Encoder) h(h uint64, v float32)    { e.sp = append(e.sp, Feature{Row: row(h), Value: v}) }

// battle is battleBlock's per-card body: "<name>|<tag>" and the decorated
// "<name>|t<b>|a<b>|s<b>|c<counters>".
func (e *Encoder) battle(c *vcard, tag int) {
	e.push(c.c.nameTag[tag], 1)
	h := addByte(c.c.hDecor, bitc(c.tapped))
	h = addByte(addByte(addByte(h, '|'), 'a'), bitc(c.attacking))
	h = addByte(addByte(addByte(h, '|'), 's'), bitc(c.sick))
	e.h(e.counterString(addByte(addByte(h, '|'), 'c'), c), 1)
}

func bitc(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

// counterString is fnvCounterString: "-" for none, else "Kind=v" sorted,
// joined by ",".
func (e *Encoder) counterString(h uint64, c *vcard) uint64 {
	if c.nctr == 0 {
		return addByte(h, '-')
	}
	first := true
	for _, k := range counterOrder {
		if c.ctrListed&(1<<k) == 0 {
			continue
		}
		n := c.ctr[k]
		if !first {
			h = addByte(h, ',')
		}
		first = false
		h = addInt(addByte(addB(h, e.pool.ctr[k]), '='), int64(n))
	}
	return h
}

// zctx is one battlefield's constant mz rows and prefixes ("mz|<z>|...").
type zctx struct {
	tapped, untapped, sick, damaged, attacking, blocked, attached, canattack, canblock uint16
	ctr, attachedPipe, crp, crt, canattackP, canblockT                                 uint64
}

func newZctx(z string) zctx {
	zh := add(off, "mz|"+z+"|")
	return zctx{
		tapped: row(add(zh, "tapped")), untapped: row(add(zh, "untapped")), sick: row(add(zh, "sick")),
		damaged: row(add(zh, "damaged")), attacking: row(add(zh, "attacking")), blocked: row(add(zh, "blocked")),
		attached: row(add(zh, "attached")), canattack: row(add(zh, "canattack")), canblock: row(add(zh, "canblock")),
		ctr: add(zh, "ctr|"), attachedPipe: add(zh, "attached|"), crp: add(zh, "cr|p"), crt: add(zh, "cr|t"),
		canattackP: add(zh, "canattack|p"), canblockT: add(zh, "canblock|t"),
	}
}

var zctxs = [2]zctx{newZctx("bf-me"), newZctx("bf-opp")}

// numKey is one seat aggregate's rows: "mz|<key>" (valued) and
// "mz|<key>=<bucket>" for each bucket 0..hi.
type numKey struct {
	row uint16
	hi  int32
	eq  []uint16
}

func newNumKey(k string, hi int32) numKey {
	h := add(off, "mz|"+k)
	n := numKey{row: row(h), hi: hi}
	for v := int32(0); v <= hi; v++ {
		n.eq = append(n.eq, row(addInt(add(h, "="), int64(v))))
	}
	return n
}

// lifeBucketHi is policynet's mzLifeBucketHi: life/2 buckets up to 61 life, Commander's 40 plus lifegain.
const lifeBucketHi int32 = 30

// bucketRows is "<prefix><v>" for v in 0..hi.
func bucketRows(prefix string, hi int32) []uint16 {
	out := make([]uint16, hi+1)
	for v := range out {
		out[v] = row(addInt(add(off, prefix), int64(v)))
	}
	return out
}

var (
	nkMeAvail, nkOppAvail                         = newNumKey("me|avail", 8), newNumKey("opp|avail", 8)
	nkMeCanAttackN, nkMeCanAttackPower            = newNumKey("me|canattack-n", 6), newNumKey("me|canattack-power", 12)
	nkOppCanBlockN, nkOppCanBlockTough            = newNumKey("opp|canblock-n", 6), newNumKey("opp|canblock-tough", 12)
	nkOppCanAttackN, nkOppCanAttackPower          = newNumKey("opp|canattack-n", 6), newNumKey("opp|canattack-power", 12)
	nkMeCanBlockN, nkMeFlyers, nkOppFlyers        = newNumKey("me|canblock-n", 6), newNumKey("me|flyers", 4), newNumKey("opp|flyers", 4)
	rowsLifediff, rowsOppLife, rowsMeLife         = bucketRows("mz|lifediff=", 14), bucketRows("mz|opp|life=", lifeBucketHi), bucketRows("mz|me|life=", lifeBucketHi)
	rowMeLethal, rowOppLethal                     = row(add(off, "mz|me|lethal-unblocked")), row(add(off, "mz|opp|lethal-unblocked"))
	rowStack                                      = [2]uint16{row(add(off, "mz|stack|me")), row(add(off, "mz|stack|opp"))}
	rowHandInstant, rowHandCastable, rowHandCastI = row(add(off, "mz|hand|instant-speed")), row(add(off, "mz|hand|castable")), row(add(off, "mz|hand|castable|instant-speed"))
	hHandCrP                                      = add(off, "mz|hand|cr|p")
)

// mz is mzState.
func (e *Encoder) mz(seat uint8, me, opp *vplayer) {
	v := &e.v
	var mine, theirs agg
	for _, ci := range me.bf {
		e.permRows(zcBfMe, &v.cards[ci], true)
		mine.add(&v.cards[ci])
	}
	for _, ci := range opp.bf {
		e.permRows(zcBfOpp, &v.cards[ci], true)
		theirs.add(&v.cards[ci])
	}
	myAvail := me.avail
	for _, ci := range me.hand {
		e.handCard(&v.cards[ci], myAvail)
	}
	e.mzStack(seat)
	e.mzGlobal(seat, me, opp, &mine, &theirs)
}

// mzStack is the mz stack rows.
func (e *Encoder) mzStack(seat uint8) {
	v := &e.v
	for i := range v.stack {
		sv := &v.stack[i]
		if sv.cv < 0 {
			continue
		}
		c := &v.cards[sv.cv]
		side := 1
		if c.ctrl == seat {
			side = 0
		}
		e.push(rowStack[side], 1)
		if len(c.c.api) > 0 {
			e.push(c.c.stackAPI[side], 1)
		}
		for _, r := range c.stackTypes(side) {
			e.push(r, 1)
		}
	}
}

// mzGlobal is the mz seat aggregates.
func (e *Encoder) mzGlobal(seat uint8, me, opp *vplayer, mine, theirs *agg) {
	e.killMine, e.killTheirs = *mine, *theirs
	myAvail := me.avail
	num := func(k *numKey, val int32) {
		e.push(k.row, float32(val))
		e.push(k.eq[clampI(val, k.hi)], 1)
	}
	num(&nkMeAvail, myAvail)
	num(&nkOppAvail, opp.avail)
	num(&nkMeCanAttackN, mine.canAttack)
	num(&nkMeCanAttackPower, mine.canAttackPower)
	num(&nkOppCanBlockN, theirs.canBlock)
	num(&nkOppCanBlockTough, theirs.canBlockTough)
	num(&nkOppCanAttackN, theirs.canAttack)
	num(&nkOppCanAttackPower, theirs.canAttackPower)
	num(&nkMeCanBlockN, mine.canBlock)
	num(&nkMeFlyers, mine.flyers)
	num(&nkOppFlyers, theirs.flyers)
	diff := me.life - opp.life
	e.push(rowsLifediff[clampI(diff/3+7, 14)], 1)
	e.push(rowsOppLife[clampI(opp.life/2, lifeBucketHi)], 1)
	e.push(rowsMeLife[clampI(me.life/2, lifeBucketHi)], 1)
	if mine.canAttackPower >= opp.life {
		e.push(rowMeLethal, 1)
	}
	if theirs.canAttackPower >= me.life {
		e.push(rowOppLethal, 1)
	}
}

func (e *Encoder) permRows(zi int, c *vcard, ctr bool) {
	zc := &zctxs[zi]
	for _, r := range c.typeRows(zi) {
		e.push(r, 1)
	}
	if c.tapped {
		e.push(zc.tapped, 1)
	} else {
		e.push(zc.untapped, 1)
	}
	if c.sick {
		e.push(zc.sick, 1)
	}
	if c.damage > 0 {
		e.push(zc.damaged, 1)
	}
	if c.attacking {
		e.push(zc.attacking, 1)
	}
	if c.blocked {
		e.push(zc.blocked, 1)
	}
	e.eachKw(c, func(k *kwInfo) { e.push(k.rows[zi], 1) })
	if ctr && c.nctr > 0 {
		for _, k := range counterOrder {
			if c.ctrListed&(1<<k) != 0 {
				e.h(addB(zc.ctr, e.pool.ctr[k]), float32(c.ctr[k]))
			}
		}
	}
	if c.attached != noObj {
		e.push(zc.attached, 1)
		if host := e.bfCard(c.attached); host != nil {
			e.push(host.c.attachedHost[zi], 1)
			e.h(addB(c.c.onPrefix, host.c.name), 1)
		}
	}
	if !c.creature() {
		return
	}
	p, t := clampI(c.power, 9), clampI(c.tough-c.damage, 9)
	crp := addInt(zc.crp, p)
	e.h(addInt(addByte(addByte(crp, '|'), 't'), t), 1)
	e.h(crp, 1)
	e.h(addInt(zc.crt, t), 1)
	e.h(addInt(addByte(addByte(addInt(c.c.nameP[zi], p), '|'), 't'), t), 1)
	e.eachKw(c, func(k *kwInfo) { e.push(k.crRows[zi], 1) })
	if !c.tapped && (!c.sick || c.hasHaste()) && !c.hasDefender() {
		e.push(zc.canattack, 1)
		e.h(addInt(zc.canattackP, p), 1)
	}
	if !c.tapped {
		e.push(zc.canblock, 1)
		e.h(addInt(zc.canblockT, t), 1)
	}
}

// agg is one battlefield's mz aggregates.
type agg struct{ canAttack, canAttackPower, canBlock, canBlockTough, flyers int32 }

func (a *agg) add(c *vcard) {
	if !c.creature() {
		return
	}
	if c.hasFlying() {
		a.flyers++
	}
	if !c.tapped && (!c.sick || c.hasHaste()) && !c.hasDefender() {
		a.canAttack++
		a.canAttackPower += c.power
	}
	if !c.tapped {
		a.canBlock++
		a.canBlockTough += c.tough - c.damage
	}
}

// handCard is policynet handCard for the viewer's hand.
func (e *Encoder) handCard(c *vcard, avail int32) {
	for _, r := range c.c.typeRows[zcHand] {
		e.push(r, 1)
	}
	e.push(c.c.handMv, 1)
	instant := c.c.instantType || c.hasFlash()
	if instant {
		e.push(rowHandInstant, 1)
	}
	if !c.c.land && c.c.mv <= avail {
		e.push(rowHandCastable, 1)
		for _, r := range c.c.castRows {
			e.push(r, 1)
		}
		if instant {
			e.push(rowHandCastI, 1)
		}
	}
	if len(c.c.api) > 0 {
		e.push(c.c.handAPI, 1)
	}
	e.eachKw(c, func(k *kwInfo) { e.push(k.rows[zcHand], 1) })
	if c.c.creature {
		e.h(addInt(addByte(addByte(addInt(hHandCrP, clampI(c.power, 9)), '|'), 't'), clampI(c.tough, 9)), 1)
	}
}

// bfCard is the battlefield card of obj o (policynet battlefieldName's
// lookup), nil when it is not on the battlefield.
func (e *Encoder) bfCard(o uint8) *vcard {
	v := &e.v
	for p := int(v.n) - 1; p >= 0; p-- {
		bf := v.players[p].bf
		for j := len(bf) - 1; j >= 0; j-- {
			if v.cards[bf[j]].obj == o {
				return &v.cards[bf[j]]
			}
		}
	}
	return nil
}

// Entity raw layout (policynet entity.go).
const (
	entTypeOff    = 0
	entPower      = 10
	entToughness  = 11
	entDamaged    = 12
	entKwOff      = 13
	entTapped     = 45
	entSick       = 46
	entCanAttack  = 47
	entCanBlock   = 48
	entAttacking  = 49
	entBlocking   = 50
	entBlocked    = 51
	entCtrPlus    = 52
	entCtrMinus   = 53
	entCtrLoyalty = 54
	entCtrOther   = 55
	entAttached   = 56
	entAttachMine = 57
	entHosts      = 58
	entMine       = 59
	entZoneBf     = 60
	entZoneHand   = 61
	entZoneStack  = 62
	entCastable   = 63
	entManaValue  = 64
	entInstant    = 65
	entFaceDown   = 66
	entOne        = 67

	// Relational slots (entity format 3: what the stack targets and, on a
	// stack card, what it targets; format 3 adds the combat slots 78-82).
	entTgtByMe      = 68
	entTgtByOpp     = 69
	entTgtPermMe    = 70
	entTgtPermOpp   = 71
	entTgtPlayerMe  = 72
	entTgtPlayerOpp = 73
	entTgtStack     = 74
	entTgtGrave     = 75
	entStackX       = 76
	entStackKicked  = 77
	entCombatFoes   = 78
	entCombatFoePow = 79
	entCombatFoeTgh = 80
	entCombatDies   = 81
	entCombatKills  = 82
	entReserved83   = 83

	// The v2 block, appended so nothing at 0..83 moves (policynet's v2
	// block, bit for bit): printed mana colour, pip counts and a coarse
	// category.
	entColourOff = 84 // W U B R G C, one each
	entPipsOff   = 90 // W U B R G generic, the pip counts / pipMax
	entCategory  = 96 // removal, creature, ramp, trick (coarse, overlapping)
)

var entityTypeMasks = [10]uint16{tbCreature, tbLand, tbArtifact, tbEnchantment, tbPlaneswalker,
	tbInstant, tbSorcery, tbBattle, tbLegendary, tbBasic}

// entities is entityCardList: my battlefield, the opponent's, my hand, the
// stack's cards; an object listed once.
func (e *Encoder) entities(seat uint8, me, opp *vplayer) {
	v := &e.v
	e.gen++
	if e.gen == 0 {
		e.seen = [256]uint32{}
		e.gen = 1
	}
	e.cards = e.cards[:0]
	one := func(ci int32, group uint8, zone int) {
		c := &v.cards[ci]
		if e.seen[c.obj] == e.gen {
			return
		}
		e.seen[c.obj] = e.gen
		e.cards = append(e.cards, Card{Group: group})
		e.entityCard(&e.cards[len(e.cards)-1], c, zone, seat, me.avail)
	}
	for _, ci := range me.bf {
		one(ci, 0, entZoneBf)
	}
	for _, ci := range opp.bf {
		one(ci, 1, entZoneBf)
	}
	for _, ci := range me.hand {
		one(ci, 2, entZoneHand)
	}
	for i := range v.stack {
		if ci := v.stack[i].cv; ci >= 0 {
			one(ci, 3, entZoneStack)
		}
	}
}

func (e *Encoder) entityCard(out *Card, c *vcard, zone int, seat uint8, avail int32) {
	raw := &out.Raw
	for i, m := range entityTypeMasks {
		if c.exact()&m != 0 {
			raw[entTypeOff+i] = 1
		}
	}
	creature := c.fold()&tbCreature != 0
	if creature || c.fold()&tbPlaneswalker != 0 || zone == entZoneHand {
		raw[entPower] = float32(c.power) / 4
		raw[entToughness] = float32(c.tough-c.damage) / 4
	}
	if c.damage > 0 {
		raw[entDamaged] = 1
	}
	e.eachKw(c, func(k *kwInfo) {
		if k.bit >= 0 {
			raw[entKwOff+int(k.bit)] = 1
		} else {
			raw[entKwOff+len(keywordBits)] = 1
		}
	})
	if zone == entZoneBf {
		if c.tapped {
			raw[entTapped] = 1
		}
		if c.sick {
			raw[entSick] = 1
		}
		if creature {
			if !c.tapped && (!c.sick || c.hasHaste()) && !c.hasDefender() {
				raw[entCanAttack] = 1
			}
			if !c.tapped {
				raw[entCanBlock] = 1
			}
		}
		if c.attacking {
			raw[entAttacking] = 1
		}
		if c.blocks != noObj {
			raw[entBlocking] = 1
		}
		if c.blocked {
			raw[entBlocked] = 1
		}
		for _, k := range counterOrder {
			n := float32(c.ctr[k])
			if n == 0 {
				continue
			}
			switch kop.CounterKind(k) {
			case kop.CtrP1P1:
				raw[entCtrPlus] += n / 2
			case kop.CtrM1M1:
				raw[entCtrMinus] += n / 2
			case kop.CtrLoyalty:
				raw[entCtrLoyalty] += n / 4
			default:
				raw[entCtrOther] += n / 2
			}
		}
		if c.attached != noObj {
			raw[entAttached] = 1
			if host := e.bfCard(c.attached); host != nil && host.ctrl == seat {
				raw[entAttachMine] = 1
			}
		}
		hosts := 0
		v := &e.v
		for p := 0; p < int(v.n); p++ {
			for _, ci := range v.players[p].bf {
				if v.cards[ci].attached == c.obj {
					hosts++
				}
			}
		}
		if hosts > 0 {
			raw[entHosts] = float32(hosts) / 2
		}
	}
	if c.ctrl == seat {
		raw[entMine] = 1
	}
	switch zone {
	case entZoneBf:
		raw[entZoneBf] = 1
	case entZoneHand:
		raw[entZoneHand] = 1
	case entZoneStack:
		raw[entZoneStack] = 1
	}
	raw[entManaValue] = float32(c.c.mv) / 5
	if c.c.fold&tbInstant != 0 || c.hasFlash() {
		raw[entInstant] = 1
	}
	if zone == entZoneHand && c.c.fold&tbLand == 0 && c.c.mv <= avail {
		raw[entCastable] = 1
	}
	raw[entOne] = 1
	// The v2 block: colour and pips are printed (pool-load time); the
	// category reads the current type line, as policynet's does.
	copy(raw[entColourOff:entCategory], c.c.colourPips[:])
	fold := c.fold()
	removal := c.c.removalAPI && fold&(tbCreature|tbLand) == 0
	if removal {
		raw[entCategory] = 1
	}
	if creature {
		raw[entCategory+1] = 1
	}
	if fold&tbLand != 0 || c.c.rampName {
		raw[entCategory+2] = 1
	}
	if fold&tbInstant != 0 && !removal {
		raw[entCategory+3] = 1
	}
	raw[entTgtByMe] = float32(c.rel.tgtMe)
	raw[entTgtByOpp] = float32(c.rel.tgtOpp)
	if zone == entZoneStack {
		raw[entTgtPermMe] = float32(c.rel.permMe)
		raw[entTgtPermOpp] = float32(c.rel.permOpp)
		raw[entTgtPlayerMe] = float32(c.rel.playerMe)
		raw[entTgtPlayerOpp] = float32(c.rel.playerOpp)
		raw[entTgtStack] = float32(c.rel.stack)
		raw[entTgtGrave] = float32(c.rel.grave)
		raw[entStackX] = float32(c.rel.x) / 5
		if c.rel.kicked {
			raw[entStackKicked] = 1
		}
	}
	if zone == entZoneBf && c.rel.combatFoes > 0 {
		raw[entCombatFoes] = float32(c.rel.combatFoes) / 2
		raw[entCombatFoePow] = float32(c.rel.combatFoePow) / 4
		raw[entCombatFoeTgh] = float32(c.rel.combatFoeTgh) / 4
		if c.rel.combatDies {
			raw[entCombatDies] = 1
		}
		if c.rel.combatKills {
			raw[entCombatKills] = 1
		}
	}
	if len(c.c.name) > 0 {
		out.Rows[out.NRows] = Feature{Row: c.c.entName, Value: 1}
		out.NRows++
	}
	if len(c.c.api) > 0 {
		out.Rows[out.NRows] = Feature{Row: c.c.entAPI, Value: 1}
		out.NRows++
	}
}

// sortScratch is stableSortByRow's buffers.
type sortScratch struct {
	idx, tmp []uint16
	out      []Feature
}

// stableSortByRow is a stable sort by row -- the order policynet's
// stableSortByRow leaves (every stable sort leaves the same one) -- as a
// two-pass LSD radix sort of indices over the 14-bit rows.
func stableSortByRow(fs []Feature, sc *sortScratch) {
	n := len(fs)
	if n < 2 {
		return
	}
	sorted := true
	for i := 1; i < n; i++ {
		if fs[i].Row < fs[i-1].Row {
			sorted = false
			break
		}
	}
	if sorted {
		return
	}
	if cap(sc.idx) < n {
		sc.idx, sc.tmp, sc.out = make([]uint16, n, 2*n), make([]uint16, n, 2*n), make([]Feature, n, 2*n)
	}
	idx, tmp, out := sc.idx[:n], sc.tmp[:n], sc.out[:n]
	var cnt [128]uint16
	for _, f := range fs {
		cnt[f.Row&127]++
	}
	pos := uint16(0)
	for b := range cnt {
		c := cnt[b]
		cnt[b] = pos
		pos += c
	}
	for i, f := range fs {
		b := f.Row & 127
		tmp[cnt[b]] = uint16(i)
		cnt[b]++
	}
	cnt = [128]uint16{}
	for _, f := range fs {
		cnt[(f.Row>>7)&127]++
	}
	pos = 0
	for b := range cnt {
		c := cnt[b]
		cnt[b] = pos
		pos += c
	}
	for _, i := range tmp {
		b := (fs[i].Row >> 7) & 127
		idx[cnt[b]] = i
		cnt[b]++
	}
	for k, i := range idx {
		out[k] = fs[i]
	}
	copy(fs, out)
}

// Reserve grows every buffer the encoder appends to to the most a kernel
// position can need, so encoding never allocates afterwards (a hot loop
// calls it once per encoder).
func (e *Encoder) Reserve() {
	const objs = dirac.MaxObj
	if cap(e.extOut) < ExtWidth {
		e.extOut = make([]Feature, 0, ExtWidth)
	}
	if cap(e.sp) < 4096 {
		e.sp = make([]Feature, 0, 4096)
	}
	if cap(e.cards) < objs {
		e.cards = make([]Card, 0, objs)
	}
	if cap(e.objs) < objs {
		e.objs = make([]dirac.Obj, 0, objs)
	}
	if cap(e.order) < objs {
		e.order = make([]uint16, 0, objs)
	}
	for i := range e.zl {
		if cap(e.zl[i]) < objs {
			e.zl[i] = make([]uint16, 0, objs)
		}
	}
	v := &e.v
	if cap(v.cards) < objs+dirac.MaxStack {
		v.cards = make([]vcard, 0, objs+dirac.MaxStack)
	}
	if cap(v.stack) < dirac.MaxStack {
		v.stack = make([]vstack, 0, dirac.MaxStack)
	}
	for p := 0; p <= len(v.players); p++ {
		pl := &v.others
		if p < len(v.players) {
			pl = &v.players[p]
		}
		pl.bf = make([]int32, 0, objs)
		pl.hand = make([]int32, 0, objs)
		pl.grave = make([]int32, 0, objs)
		pl.exile = make([]int32, 0, objs)
	}
	e.sorts.idx = make([]uint16, 0, 4096)
	e.sorts.tmp = make([]uint16, 0, 4096)
	e.sorts.out = make([]Feature, 0, 4096)
	e.opt.refs = make([]cref, 0, objs+dirac.MaxStack)
	e.tym.reserve()
	for i := range e.parts {
		if e.parts[i] == nil {
			e.parts[i] = newPartsSeat()
		}
	}
	e.vbase()
}
