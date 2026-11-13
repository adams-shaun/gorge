package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// target.go: botpolicy.chooseTargets (R1-R5) for one kernel target slot.
// gorge poses each targeted SA's targets as one decision; the kernel poses
// one slot at a time, so the port takes gorge's first pick: the ranked
// foreign targets first (own first for a one-way boon grant), ties on
// gorge's option order (players, then objects zone by zone, seat by seat).
// The slot's TargetEffect comes from Facts (the targeted SAs of the
// ability's chain; a charm's chosen modes only), matched to the slot as
// the sbt agent matches it (the slot's index, then the ValidTgts$ fit).

const (
	tierThreat     int32 = 1_000_000
	tierFaceLethal int32 = 800_000
	tierValue      int32 = 600_000
)

// threat is target.go's Creature.threat.
func (c *creature) threat() int32 {
	v := c.power*3 + c.remTough()*2
	if !c.tapped {
		v += 6
	}
	if c.has(kop.KwFlying) {
		v += 16
	}
	if c.has(kop.KwDeathtouch) {
		v += 12
	}
	if c.has(kop.KwFirstStrike) {
		v += 8
	}
	if c.has(kop.KwVigilance) {
		v += 6
	}
	if c.has(kop.KwReach) {
		v += 4
	}
	return v
}

type tgtRank struct {
	score int32
	idx   int16 // gorge option index (position in the sorted list)
	opt   int16 // kernel option index
	ours  bool
	key   uint32
}

func (bt *Bot) target(s *dirac.State, d *dirac.Decision) int {
	b := &bt.b
	b.fill(s, bt.f, d.Player)
	me := d.Player
	te, pend := bt.targetEffect(s, d)

	// gorge's option list, in its order (NoTarget is gorge's done)
	rs := &bt.tr
	n := 0
	// botpolicy's self-host skip: an up-to removal (the slot offers
	// NoTarget, gorge's Min reached) never takes its own host, whose exile
	// "until this leaves" would return host and exiles at once
	skipHost := dirac.NoObj
	if te != nil && te.Remove != RmNone {
		skipHost = s.FeatSource()
	}
	noTarget, skipped := -1, false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkTarget && a.Target() == dirac.NoTarget && noTarget < 0 {
			noTarget = i
		}
	}
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkTarget || a.Target() == dirac.NoTarget {
			continue
		}
		if noTarget >= 0 && skipHost != dirac.NoObj && dirac.IsObjTarget(a.Target()) && dirac.Obj(a.Target()) == skipHost {
			skipped = true
			continue
		}
		rs[n] = tgtRank{opt: int16(i), key: bt.targetKey(s, uint8(a.Target()))}
		n++
	}
	if n == 0 {
		if skipped {
			return noTarget
		}
		return 0
	}
	for i := 1; i < n; i++ {
		r := rs[i]
		j := i
		for j > 0 && rs[j-1].key > r.key {
			rs[j] = rs[j-1]
			j--
		}
		rs[j] = r
	}
	mode := 0 // 0 rankOption, 1 effect damage, 2 removal
	var dmg int32
	leadOwn, ownAsc := false, false
	spare := false
	switch {
	case te != nil && (te.API == ApiDamage || te.API == ApiDamageAll) && bt.damageOf(s, te) >= 0:
		mode, dmg = 1, bt.damageOf(s, te)
		spare = b.hasSpareManaAfter(pend)
	case te != nil && (te.API == ApiCounter || te.Remove != RmNone):
		mode = 2
	case te != nil && te.API == ApiAttach:
		// attachrules.go's attachRanking under the default AuraPolarity: a
		// boon Aura leads with the own board, a curse with the opposing one and
		// falls back to the weakest own permanent
		leadOwn, ownAsc = te.AttachPol > 0, te.AttachPol < 0
	case te != nil && te.Boon:
		leadOwn = true
	}
	for i := 0; i < n; i++ {
		r := &rs[i]
		r.idx = int16(i)
		tg := d.Options[r.opt].Target()
		switch mode {
		case 1:
			bt.effectRank(r, uint8(tg), me, dmg, spare)
		case 2:
			bt.removalRank(r, uint8(tg), me, te)
		default:
			bt.rankOption(r, uint8(tg), me)
		}
	}
	// the first pick: the best of the leading group (stable, by index)
	best := -1
	for pass := 0; pass < 2 && best < 0; pass++ {
		wantOurs := leadOwn == (pass == 0)
		for i := 0; i < n; i++ {
			r := &rs[i]
			if r.ours != wantOurs {
				continue
			}
			if best < 0 {
				best = i
				continue
			}
			if leadOwn && pass == 1 {
				// byScoreAsc over the foreign targets: least harm first, a
				// face last
				hi, hb := bt.harm(d, r), bt.harm(d, &rs[best])
				if hi < hb {
					best = i
				}
				continue
			}
			if ownAsc && pass == 1 {
				if r.score < rs[best].score {
					best = i
				}
				continue
			}
			if r.score > rs[best].score {
				best = i
			}
		}
	}
	if best < 0 {
		return 0
	}
	return int(rs[best].opt)
}

func (bt *Bot) harm(d *dirac.Decision, r *tgtRank) int32 {
	if d.Options[r.opt].Target() >= dirac.PlayerTarget0 {
		return 1 << 30
	}
	return r.score
}

// targetKey is a target's place in gorge's option list: players first,
// then objects by zone class (stack, battlefield, graveyard, exile, hand,
// library), seat, zone position.
func (bt *Bot) targetKey(s *dirac.State, tg uint8) uint32 {
	if dirac.TCode(tg) >= dirac.PlayerTarget0 {
		return uint32(dirac.TCode(tg) - dirac.PlayerTarget0)
	}
	if i, ok := dirac.StackTargetIndex(dirac.TCode(tg)); ok {
		return uint32(i) // a stack ability: the stack class, by its place from the bottom
	}
	o := dirac.Obj(tg)
	var zone, seat, idx uint32
	switch s.ObjZone(o) {
	case kop.ZoneStack:
		zone = 0
		for i := 0; i < s.StackLen(); i++ {
			if _, src, _, _ := s.StackEntry(i); src == o {
				idx = uint32(i)
			}
		}
	case kop.ZoneBattlefield:
		zone, seat, idx = 1, uint32(s.Controller(o)), uint32(bt.b.bfPos[o])
	case kop.ZoneGraveyard:
		zone, seat, idx = 2, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	case kop.ZoneExile:
		zone, seat, idx = 3, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	case kop.ZoneHand:
		zone, seat, idx = 4, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	default:
		zone, seat, idx = 5, uint32(s.ObjOwner(o)), uint32(s.FeatZoneIndex(o))
	}
	return 1<<24 | zone<<20 | (seat%dirac.MaxSeats)<<16 | idx
}

// optPlayer is the gorge option's Player: the player itself, or the
// object's zone's player (controller on the battlefield and the stack,
// owner elsewhere).
func optPlayer(s *dirac.State, tg uint8) uint8 {
	if dirac.TCode(tg) >= dirac.PlayerTarget0 {
		return uint8(dirac.TCode(tg) - dirac.PlayerTarget0)
	}
	if i, ok := dirac.StackTargetIndex(dirac.TCode(tg)); ok {
		if i < s.StackLen() {
			_, _, _, ctrl := s.StackEntry(i)
			return ctrl
		}
		return 0
	}
	o := dirac.Obj(tg)
	switch s.ObjZone(o) {
	case kop.ZoneBattlefield:
		return s.Controller(o)
	case kop.ZoneStack:
		for i := 0; i < s.StackLen(); i++ {
			if _, src, _, ctrl := s.StackEntry(i); src == o {
				return ctrl
			}
		}
	}
	return s.ObjOwner(o)
}

// rankOption is target.go's rankOption.
func (bt *Bot) rankOption(r *tgtRank, tg uint8, me uint8) {
	b := &bt.b
	pl := optPlayer(b.s, tg)
	if dirac.TCode(tg) >= dirac.PlayerTarget0 {
		r.score, r.ours = 0, pl == me
		return
	}
	c := b.creOf(dirac.Obj(tg))
	if c != nil {
		r.score = c.threat()
	}
	r.ours = pl == me
}

// effectRank is target.go's effectRankerAfter.
func (bt *Bot) effectRank(r *tgtRank, tg uint8, me uint8, dmg int32, spare bool) {
	b := &bt.b
	pl := optPlayer(b.s, tg)
	if dirac.TCode(tg) >= dirac.PlayerTarget0 {
		if pl == me {
			r.score, r.ours = 0, true
			return
		}
		if b.life[pl] <= dmg {
			r.score = tierFaceLethal
		}
		return
	}
	c := b.creOf(dirac.Obj(tg))
	if pl == me {
		if c != nil {
			r.score = c.threat()
		}
		r.ours = true
		return
	}
	if c == nil {
		return
	}
	if c.remTough() <= dmg {
		if c.power > 0 && !c.tapped && c.power >= b.life[me] {
			r.score = tierThreat + c.threat()
			return
		}
		if spare {
			r.score = tierValue + c.threat()
			return
		}
	}
	r.score = c.threat()
}

// removalRank is target.go's removalRanker.
func (bt *Bot) removalRank(r *tgtRank, tg uint8, me uint8, te *TgtEff) {
	b := &bt.b
	bt.rankOption(r, tg, me)
	if r.ours {
		return
	}
	if te.API == ApiCounter {
		if dirac.IsObjTarget(dirac.TCode(tg)) {
			o := dirac.Obj(tg)
			for i := 0; i < b.nStack; i++ {
				if e := &b.stack[i]; e.obj == o && e.isSpell && e.ctrl != me {
					r.score = tierValue + e.cmc
					return
				}
			}
		}
		return
	}
	if dirac.TCode(tg) >= dirac.PlayerTarget0 {
		return
	}
	value := r.score
	if value == 0 && dirac.IsObjTarget(dirac.TCode(tg)) {
		if i := b.cardX[tg]; i > 0 {
			value = b.card[i-1].cmc * 3
		}
	}
	if value == 0 {
		return
	}
	var bonus int32
	switch te.Remove {
	case RmExile, RmCommand:
		bonus = 30
	case RmLibrary:
		bonus = 20
	case RmGraveyard, RmDestroy, RmSacrifice:
		bonus = 10
	case RmBounce:
		bonus = 5
	}
	r.score = tierValue + value + bonus
}

// pendCost is the pending spell's printed cost a cast-time target ask
// precedes (target.go's pendingSpellCost): pips, mana value, X.
type pendCost struct {
	ok   bool
	pips [5]int32
	cmc  int32
	hasX bool
}

// targetEffect is the asked slot's TargetEffect facts (nil: none) and the
// pending spell cost when the ask is a spell's cast-time one.
func (bt *Bot) targetEffect(s *dirac.State, d *dirac.Decision) (*TgtEff, pendCost) {
	var pc pendCost
	card, ab, _, ok := s.Building()
	if !ok {
		return nil, pc
	}
	tbl := bt.f.Tbl
	var chain []TgtEff
	cf := bt.f.card(card)
	if int(card) < len(tbl.Cards) && int(ab) < len(tbl.Cards[card].Abilities) {
		chain = cf.Abilities[ab].Tgt
		if tbl.Cards[card].Abilities[ab].Kind == kop.AbSpell {
			pc.ok = true
		}
	} else if ab == 255 {
		chain = cf.SpellTgt
		pc.ok = true
	}
	if pc.ok {
		for k := 0; k < 5; k++ {
			pc.pips[k] = int32(cf.Pips[k])
		}
		pc.cmc, pc.hasX = int32(cf.CMC), cf.HasX
	}
	if len(chain) == 0 {
		return nil, pc
	}
	modes := s.BuildModes()
	slot, _ := s.BuildTargetSlot()
	k := 0
	var last *TgtEff
	for i := range chain {
		e := &chain[i]
		if e.Mode >= 0 && modes&(1<<uint(e.Mode)) == 0 {
			continue
		}
		last = e
		if k >= slot && bt.slotFits(s, d, e) {
			return e, pc
		}
		k++
	}
	return last, pc
}

// slotFits is sbtSlotFits: every offered target fits the SA's ValidTgts$
// controller qualifiers.
func (bt *Bot) slotFits(s *dirac.State, d *dirac.Decision, e *TgtEff) bool {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkTarget || a.Target() == dirac.NoTarget {
			continue
		}
		tg := a.Target()
		mine := optPlayer(s, uint8(tg)) == d.Player
		var need uint8
		switch {
		case tg >= dirac.PlayerTarget0 && mine:
			need = FitPlayerMine
		case tg >= dirac.PlayerTarget0:
			need = FitPlayerOpp
		case mine:
			need = FitMine
		default:
			need = FitOpp
		}
		if e.Fit&need == 0 {
			return false
		}
	}
	return true
}

// hasSpareManaAfter is target.go's hasSpareManaAfter: after the pending
// payment, the pool plus the untapped single-colour basic sources still
// pay the cheapest instant-speed card even with any one unit (or source)
// spent.
func (b *board) hasSpareManaAfter(pend pendCost) bool {
	var avail [6]int32
	copy(avail[:], b.pool[:])
	var srcs [dirac.MaxPerm][6]int32
	ns := 0
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.onBF || !c.basic || c.tapped || c.f.ProdAny || c.f.ProdIndet {
			continue
		}
		var src [6]int32
		colours := 0
		for k := 0; k < 6; k++ {
			if c.f.Produces[k] > 0 {
				colours++
				src[k] = int32(c.f.Produces[k])
			}
		}
		if colours != 1 {
			continue
		}
		for k := 0; k < 6; k++ {
			avail[k] += src[k]
		}
		if ns < len(srcs) {
			srcs[ns] = src
			ns++
		}
	}
	if total6(avail) == 0 {
		return false
	}
	minCost := int32(-1)
	var need [5]int32 // the reserve cards' pips, each checked separately below
	var rs [dirac.MaxObj]*cardF
	nr := 0
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.castable || !c.instant || c.cmc <= 0 {
			continue
		}
		if minCost < 0 || c.cmc < minCost {
			minCost, nr = c.cmc, 0
		}
		if c.cmc == minCost {
			rs[nr] = c
			nr++
		}
	}
	if minCost < 0 {
		return true
	}
	pays := func(m [6]int32) bool {
		if total6(m) < minCost {
			return false
		}
		for j := 0; j < nr; j++ {
			for k := 0; k < 5; k++ {
				if m[k] < int32(rs[j].f.Pips[k]) {
					return false
				}
			}
		}
		return true
	}
	if pend.ok {
		for j := 0; j < nr; j++ {
			for k := 0; k < 5; k++ {
				if p := int32(rs[j].f.Pips[k]); p > need[k] {
					need[k] = p
				}
			}
		}
		rem, ok := pendingSpend(avail, pend, need)
		if !ok {
			return false
		}
		avail = rem
	}
	if !pays(avail) {
		return false
	}
	for k := 0; k < 6; k++ {
		if b.pool[k] > 0 && avail[k] > 0 {
			left := avail
			left[k]--
			if !pays(left) {
				return false
			}
		}
	}
	for j := 0; j < ns; j++ {
		left := avail
		cover := true
		for k := 0; k < 6; k++ {
			if avail[k] < srcs[j][k] {
				cover = false
				break
			}
			left[k] -= srcs[j][k]
		}
		if !cover {
			continue
		}
		if !pays(left) {
			return false
		}
	}
	return true
}

func total6(m [6]int32) int32 {
	var n int32
	for _, x := range m {
		n += x
	}
	return n
}

// pendingSpend is target.go's pendingSpendWorstCase: the pending cost's
// coloured pips, then its generic part from the colour with the least
// slack over the reserve's pips.
func pendingSpend(avail [6]int32, pend pendCost, need [5]int32) ([6]int32, bool) {
	if pend.hasX {
		return [6]int32{}, false
	}
	rem := avail
	for k := 0; k < 5; k++ {
		if rem[k] < pend.pips[k] {
			return [6]int32{}, false
		}
		rem[k] -= pend.pips[k]
	}
	generic := pend.cmc
	for k := 0; k < 5; k++ {
		generic -= pend.pips[k]
	}
	if generic < 0 {
		generic = 0
	}
	if total6(rem) < generic {
		return [6]int32{}, false
	}
	for ; generic > 0; generic-- {
		pick := -1
		for k := 0; k < 5; k++ {
			if rem[k] == 0 {
				continue
			}
			if pick < 0 || rem[k]-need[k] < rem[pick]-need[pick] {
				pick = k
			}
		}
		if pick < 0 {
			return [6]int32{}, false
		}
		rem[pick]--
	}
	return rem, true
}

// damageOf is the target ask's damage amount (-1 unknown): the literal,
// the kicked one, or the announced X.
func (bt *Bot) damageOf(s *dirac.State, te *TgtEff) int32 {
	kicked, x, ok := s.BuildingCast()
	switch {
	case te.DmgX:
		if !ok {
			return -1
		}
		return int32(x)
	case kicked && ok:
		return int32(te.DmgK)
	}
	return int32(te.Dmg)
}
