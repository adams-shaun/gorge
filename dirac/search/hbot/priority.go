package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// priority.go: heuristic.Bot.decide at a priority decision under auto-pay
// (paymentIntent / wantsManual / decideWithoutAutoPay) over
// botpolicy.decide's KPriority arm: chooseLand, chooseCast, chooseAbility,
// pass. The candidate list is gorge's, built from the kernel's options:
//
//   - a land play: always listed;
//   - a cast the floating pool pays (FeatPoolPays): a legacy option, plan
//     backed when gorge's planner also pays it in the same mode
//     (FeatPlannedCast; plain or flashback);
//   - a plain or flashback cast only the planner pays: one plan-backed
//     entry appended after every legacy option (paymentIntent's plan-only
//     entries), unless the object already has a legacy ordinary option;
//   - an activation gorge lists ("ability": FeatPlannedActivation or
//     FeatAbilityPoolPays), not a scripted one (FeatScripted);
//   - pass.
//
// Any other kernel cast (kicked with the pool short, {X} spells, either-or
// costs) is not on gorge's list. On the seat's own turn outside a main
// phase the wrapper declines and the plain policy sees the legacy options
// only. gorge's manual path (wantsManual: a castable card no plan pays)
// taps mana in a main phase until nothing more helps and then plays its
// ordinary policy over the floated pool; the kernel has no manual mana, so
// the port plays those taps out on its board (floatByHand) and ranks what
// the floated pool pays.

// cand is one candidate of the private priority list.
type cand struct {
	opt     int16 // index in d.Options
	key     uint32
	kind    dirac.ActionKind
	obj     dirac.Obj
	mode    uint8 // cast mode bits / ability index
	planned bool  // PlanBacked
	offHand bool  // the object is outside its owner's hand
}

const (
	secHand    = 0
	secGrave   = 1
	secAbility = 3
	secPass    = 4
	secSynth   = 7
)

func (bt *Bot) priority(s *dirac.State, d *dirac.Decision) int {
	b := &bt.b
	b.fillPriority(s, bt.f, d.Player, d)
	for i := 0; i < int(d.N); i++ {
		bt.pvC[i], bt.ppC[i] = 0, 0
	}
	me := d.Player
	n := 0
	cs := &bt.pc
	if b.myTur && !b.isMain {
		if bt.hold && bt.timedTapWould(s, d) {
			n = bt.allCands(s, d, cs)
			return bt.decidePriority(d, cs[:n])
		}
		// the wrapper declines: the plain policy over the legacy options
		n = bt.legacyCands(s, d, cs, false)
		return bt.decidePriority(d, cs[:n])
	}
	// payable: the planner-paid casts (not a dead counter)
	np := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkCast || a.Mode()&1 != 0 {
			continue
		}
		o := dirac.Obj(a.Source())
		if !bt.plannedVisibleAt(s, d, i) {
			continue
		}
		if b.cardOf(o).counter && !b.foreignSpell(me) {
			continue
		}
		bt.payable[np] = o
		np++
	}
	if bt.wantsManual(s, d, np) {
		if b.isMain && bt.tapWould(s, d) {
			bt.floatByHand(s, d, np)
			n = bt.floatedCands(s, d, cs, np)
			return bt.decidePriority(d, cs[:n])
		}
		if !b.isMain && bt.hold && bt.timedTapWould(s, d) {
			n = bt.allCands(s, d, cs)
			return bt.decidePriority(d, cs[:n])
		}
		n = bt.legacyCands(s, d, cs, false)
		return bt.decidePriority(d, cs[:n])
	}
	n = bt.legacyCands(s, d, cs, true)
	// plan-only entries, after every legacy option, in the planner's order
	start := n
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkCast || a.Mode()&1 != 0 || bt.poolPaysAt(s, d, i) {
			continue
		}
		if !bt.plannedVisibleAt(s, d, i) {
			continue
		}
		o := dirac.Obj(a.Source())
		if bt.legacyOrdinary(cs[:start], o) {
			continue
		}
		sub := uint32(0)
		z := s.ObjZone(o)
		if a.Mode()&2 == 0 && z != kop.ZoneHand {
			sub = 2
		}
		if a.Mode()&2 != 0 {
			sub = 3
		}
		cs[n] = cand{opt: int16(i), key: secSynth<<24 | sub<<16 | uint32(s.FeatZoneIndex(o))<<8, kind: dirac.AkCast, obj: o, mode: a.Mode(), planned: true}
		n++
	}
	sortCands(cs[start:n])
	return bt.decidePriority(d, cs[:n])
}

// plannedVisible reports whether cast a is one gorge's planner offers as a
// payment action (FeatPlannedCast, not a scripted play, an untyped
// permission's).
func (bt *Bot) plannedVisible(s *dirac.State, me uint8, a dirac.Action) bool {
	o := dirac.Obj(a.Source())
	if !planShapedCast(s, a) || !s.FeatPlannedCast(me, o, a.Mode()) {
		return false
	}
	if a.Mode()&dirac.FeatCastMayPlay != 0 && !s.FeatMayPlayUntyped(me, o) {
		return false
	}
	return !s.FeatScripted(me, a)
}

// planShapedCast reports whether cast a is one of the shapes gorge's
// paymentActionsForPriority builds a plan for (rules payment_plan.go): the
// plain cast, the primary flashback cast and the untyped may-play cast. A face
// cast, a warped, free, impended, evoked or alternative-cost cast and a
// flashback relative (harmonize, mayhem, beam me up) stay out of its planned list.
func planShapedCast(s *dirac.State, a dirac.Action) bool {
	m := a.Mode()
	if m&^(2|dirac.FeatCastMayPlay) != 0 {
		return false
	}
	o := dirac.Obj(a.Source())
	if m&2 != 0 {
		return !s.FeatHarmonizeCard(o) && !s.FeatMayhemCard(o) && a.Target() == 0
	}
	return m != 0 || a.Target() == 0 || s.ObjZone(o) != kop.ZoneHand
}

// plannedVisibleAt is plannedVisible of option i, memoised for the
// current priority decision (priority resets pvC).
func (bt *Bot) plannedVisibleAt(s *dirac.State, d *dirac.Decision, i int) bool {
	if v := bt.pvC[i]; v != 0 {
		return v == 2
	}
	r := bt.plannedVisible(s, d.Player, d.Options[i])
	bt.pvC[i] = 1
	if r {
		bt.pvC[i] = 2
	}
	return r
}

// poolPaysAt is FeatPoolPays of cast option i, memoised like
// plannedVisibleAt (ppC).
func (bt *Bot) poolPaysAt(s *dirac.State, d *dirac.Decision, i int) bool {
	if v := bt.ppC[i]; v != 0 {
		return v == 2
	}
	r := castPoolPays(s, d.Player, d.Options[i], nil)
	bt.ppC[i] = 1
	if r {
		bt.ppC[i] = 2
	}
	return r
}

// castPoolPays reports whether gorge's legal walk lists cast a: the pool, plus
// any mana floated by hand (extra), pays the cast's own price.
func castPoolPays(s *dirac.State, me uint8, a dirac.Action, extra *[6]int32) bool {
	o := dirac.Obj(a.Source())
	switch m := a.Mode(); {
	case m&dirac.FeatCastFree != 0:
		return true // nothing to pay
	case m&dirac.FeatCastFace != 0:
		return s.FeatFacePoolPays(me, o, a.Target(), extra)
	default:
		return s.FeatPoolPaysPlus(me, o, m, extra)
	}
}

// legacyOrdinary reports whether the legacy list has an ordinary cast of o
// (mode "" or its plan's mode): paymentIntent's legacyOrdinary.
func (bt *Bot) legacyOrdinary(cs []cand, o dirac.Obj) bool {
	for i := range cs {
		c := &cs[i]
		if c.kind == dirac.AkCast && c.obj == o && (c.mode&1 == 0) && (c.mode&2 == 0 || c.planned) {
			return true
		}
	}
	return false
}

// legacyCands fills cs with gorge's decision.Options (mana taps aside) in
// gorge's order: land plays and pool-paid casts by hand position, pool-paid
// flashback casts, the listed abilities, pass. planned marks a pool-paid
// cast plan backed when its planner pays it too (paymentIntent's matched).
func (bt *Bot) legacyCands(s *dirac.State, d *dirac.Decision, cs *[dirac.MaxOptions]cand, planned bool) int {
	b := &bt.b
	me := d.Player
	n := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		o := dirac.Obj(a.Source())
		c := cand{opt: int16(i), kind: a.Kind(), obj: o, mode: a.Mode()}
		switch a.Kind() {
		case dirac.AkPass:
			c.key = secPass << 24
		case dirac.AkPlayLand:
			if s.ObjZone(o) != kop.ZoneHand {
				c.key = secHand<<24 | 1<<20 | uint32(s.FeatZoneIndex(o))<<8
			} else {
				c.key = secHand<<24 | uint32(s.FeatZoneIndex(o))<<8
			}
		case dirac.AkCast:
			c.offHand = s.ObjZone(o) != kop.ZoneHand
			if !bt.poolPaysAt(s, d, i) || s.FeatScripted(me, a) {
				continue
			}
			z := s.ObjZone(o)
			if a.Mode()&2 != 0 {
				c.key = secGrave<<24 | uint32(s.FeatZoneIndex(o))<<8
			} else {
				sub := uint32(0)
				if z != kop.ZoneHand {
					sub = 2
				}
				c.key = secHand<<24 | sub<<20 | uint32(s.FeatZoneIndex(o))<<8 | 1 + uint32(a.Mode()&1)
			}
			if planned && a.Mode()&1 == 0 && bt.plannedVisibleAt(s, d, i) {
				c.planned = true
			}
		case dirac.AkActivate:
			if s.FeatScripted(me, a) {
				continue
			}
			if bt.notRanked(s, o, a.Mode()) || !s.FeatPlannedActivation(me, o, a.Mode()) && !s.FeatAbilityPoolPays(me, o, a.Mode(), dirac.Obj(a.Target())) {
				continue
			}
			c.key = bt.abilityKey(s, o, a.Mode())
		default:
			continue
		}
		cs[n] = c
		n++
	}
	_ = b
	sortCands(cs[:n])
	return n
}

// notRanked reports whether ability ab is one gorge lists under another option
// kind, which chooseAbility never ranks: "station" (rules/station.go) and
// "granted", a max speed ability or a grant reaching off the battlefield.
func (bt *Bot) notRanked(s *dirac.State, o dirac.Obj, ab uint8) bool {
	if bt.f.Tbl == nil {
		return false
	}
	if ab&0x80 != 0 {
		return s.ObjZone(o) != kop.ZoneBattlefield
	}
	abs := bt.f.Tbl.Cards[s.ObjCard(o)].Abilities
	if int(ab) >= len(abs) {
		return false
	}
	a := &abs[ab]
	return a.AFlags&kop.AfStation != 0 || a.Kind == kop.AbActivated && a.Gate.Ctr == kop.CtrMaxSpeed
}

// allCands is the manual path's list after its taps: every kernel play
// (each affordable with the seat's mana), unplanned, in gorge's order.
func (bt *Bot) allCands(s *dirac.State, d *dirac.Decision, cs *[dirac.MaxOptions]cand) int {
	n := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		o := dirac.Obj(a.Source())
		c := cand{opt: int16(i), kind: a.Kind(), obj: o, mode: a.Mode()}
		switch a.Kind() {
		case dirac.AkPass:
			c.key = secPass << 24
		case dirac.AkPlayLand:
			c.key = secHand<<24 | uint32(s.FeatZoneIndex(o))<<8
		case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire:
			c.offHand = s.ObjZone(o) != kop.ZoneHand
			if a.Mode()&2 != 0 {
				c.key = secGrave<<24 | uint32(s.FeatZoneIndex(o))<<8
			} else {
				c.key = secHand<<24 | uint32(s.FeatZoneIndex(o))<<8 | 1 + uint32(a.Mode()&1)
			}
		case dirac.AkActivate:
			c.key = bt.abilityKey(s, o, a.Mode())
		default:
			continue
		}
		cs[n] = c
		n++
	}
	sortCands(cs[:n])
	return n
}

// floatByHand plays out heuristic.Bot's manual path on the board: each round the
// wrapper's wantsManual asks again, then tap.go's chooseTap taps toward its
// intended card, until the wrapper hands back to the plan path or the gate stops.
// The mana lands in b.pool and bt.floated, the sources read as tapped.
func (bt *Bot) floatByHand(s *dirac.State, d *dirac.Decision, np int) {
	b := &bt.b
	bt.floated = [6]int32{}
	for n := 0; n < b.nCard; n++ {
		off := bt.offeredColours(s, d.Player)
		c := bt.bestUnpayable(d, off)
		if c == nil || !bt.wantsManual(s, d, np) {
			return
		}
		src := bt.tapToward(s, c)
		if src == nil {
			return
		}
		src.tapped = true
		if i := b.creX[src.obj]; i > 0 {
			b.cre[i-1].tapped = true
		}
		if k := bt.floatColour(d, src); k >= 0 {
			b.pool[k] += int32(src.f.Produces[k])
			bt.floated[k] += int32(src.f.Produces[k])
		}
	}
}

// floatColour is tap.go's chooseManaColour on a source tapped by hand: the first
// colour the card the source can help still needs, else its first mana. An option
// with a cost is left out, since paying it moves the pool the ask reads.
func (bt *Bot) floatColour(d *dirac.Decision, src *cardF) int8 {
	b := &bt.b
	var off [5]bool
	for k := range off {
		off[k] = src.f.PlainMana&(1<<k) != 0
	}
	if c := bt.bestUnpayableOf(d, off, false); c != nil {
		for k := range off {
			if off[k] && int32(c.f.Pips[k]) > b.pool[k] {
				return int8(k)
			}
		}
	}
	return src.f.FirstMana
}

// tapToward is tap.go's tapToward: the source that best advances c. Tier 0 makes
// a colour c still needs, or may (reflected, any); tier 1 makes other mana;
// tier 2 makes none it can show. Within a tier the fewest colours first, then the
// earlier option. A source that spends pool mana without tapping is a converter,
// which the tap gate takes only when it provably helps; the port never does.
func (bt *Bot) tapToward(s *dirac.State, c *cardF) *cardF {
	b := &bt.b
	var best *cardF
	bestTier, bestFlex := 3, 0
	for i := 0; i < b.nCard; i++ {
		x := &b.card[i]
		if !x.own || !x.onBF || x.tapped || x.f.Converter || !s.FeatManaActivatable(x.obj) {
			continue
		}
		matches, shown := false, false
		for k := 0; k < 6; k++ {
			if x.f.Produces[k] > 0 {
				shown = true
			}
			if k < 5 && int32(c.f.Pips[k]) > b.pool[k] && (x.f.Produces[k] > 0 || x.f.ProdReflected || x.f.ProdAny) {
				matches = true
			}
		}
		tier := 2
		if matches {
			tier = 0
		} else if shown {
			tier = 1
		}
		flex := x.f.distinct()
		if best == nil || tier < bestTier || tier == bestTier && flex < bestFlex {
			best, bestTier, bestFlex = x, tier, flex
		}
	}
	return best
}

// floatedCands is the priority list after floatByHand: the land plays, the casts
// the floated pool pays, the plan-backed casts the sources still pay, the
// abilities gorge lists whose own tap cost is still open, and pass.
func (bt *Bot) floatedCands(s *dirac.State, d *dirac.Decision, cs *[dirac.MaxOptions]cand, np int) int {
	b := &bt.b
	n := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		o := dirac.Obj(a.Source())
		c := cand{opt: int16(i), kind: a.Kind(), obj: o, mode: a.Mode()}
		switch a.Kind() {
		case dirac.AkPass:
			c.key = secPass << 24
		case dirac.AkPlayLand:
			c.key = secHand<<24 | uint32(s.FeatZoneIndex(o))<<8
		case dirac.AkCast:
			c.offHand = s.ObjZone(o) != kop.ZoneHand
			paid := castPoolPays(s, d.Player, a, &bt.floated)
			if !paid && (np == 0 || !bt.plannedVisibleAt(s, d, i) || !b.stillPays(b.cardOf(o))) {
				continue
			}
			c.planned = !paid
			if a.Mode()&2 != 0 {
				c.key = secGrave<<24 | uint32(s.FeatZoneIndex(o))<<8
			} else {
				c.key = secHand<<24 | uint32(s.FeatZoneIndex(o))<<8 | 1 + uint32(a.Mode()&1)
			}
		case dirac.AkActivate:
			if s.FeatScripted(d.Player, a) || bt.notRanked(s, o, a.Mode()) || b.cardOf(o).tapped && bt.f.ab(s.ObjCard(o), a.Mode()).TapCost {
				continue
			}
			if !s.FeatPlannedActivation(d.Player, o, a.Mode()) && !s.FeatAbilityPoolPays(d.Player, o, a.Mode(), dirac.Obj(a.Target())) {
				continue
			}
			c.key = bt.abilityKey(s, o, a.Mode())
		default:
			continue
		}
		cs[n] = c
		n++
	}
	sortCands(cs[:n])
	return n
}

// stillPays reports whether the pool and the untapped sources cover c's printed
// cost and pips: the planner's verdict after the taps.
func (b *board) stillPays(c *cardF) bool {
	if c.f == nil || c.cmc > b.producible() {
		return false
	}
	pips := b.pool
	for i := 0; i < b.nCard; i++ {
		x := &b.card[i]
		if x.own && x.onBF && !x.tapped {
			for k := 0; k < 5; k++ {
				pips[k] += int32(x.f.Produces[k])
			}
		}
	}
	for k := 0; k < 5; k++ {
		if int32(c.f.Pips[k]) > pips[k] {
			return false
		}
	}
	return true
}

// abilityKey is an ability option's place in gorge's battlefield walk.
func (bt *Bot) abilityKey(s *dirac.State, o dirac.Obj, ab uint8) uint32 {
	z := s.ObjZone(o)
	var zr, idx uint32
	seat := uint32(s.ObjOwner(o))
	switch z {
	case kop.ZoneBattlefield:
		zr, idx, seat = 0, uint32(bt.b.bfPos[o]), uint32(s.Controller(o))
	case kop.ZoneStack:
		zr = 1
	case kop.ZoneGraveyard:
		zr, idx = 2, uint32(s.FeatZoneIndex(o))
	case kop.ZoneHand:
		zr, idx = 3, uint32(s.FeatZoneIndex(o))
	case kop.ZoneExile:
		zr, idx = 4, uint32(s.FeatZoneIndex(o))
	default:
		zr = 5
	}
	return secAbility<<24 | zr<<20 | (seat%dirac.MaxSeats)<<16 | idx<<8 | uint32(ab)
}

func sortCands(cs []cand) {
	for i := 1; i < len(cs); i++ {
		c := cs[i]
		j := i
		for j > 0 && cs[j-1].key > c.key {
			cs[j] = cs[j-1]
			j--
		}
		cs[j] = c
	}
}

// decidePriority is botpolicy.decide's KPriority arm over the candidate
// list (candidate index = gorge's option Index): the land, the cast, the
// ability in a main phase, else pass.
func (bt *Bot) decidePriority(d *dirac.Decision, cs []cand) int {
	b := &bt.b
	if i := bt.chooseLand(cs); i >= 0 {
		return int(cs[i].opt)
	}
	if i := bt.chooseCast(d.Player, cs); i >= 0 {
		return int(cs[i].opt)
	}
	if b.isMain {
		if i := bt.chooseAbility(d.Player, cs); i >= 0 {
			return int(cs[i].opt)
		}
	}
	for i := range cs {
		if cs[i].kind == dirac.AkPass {
			return int(cs[i].opt)
		}
	}
	return passIndex(d)
}

// chooseLand is cast.go's chooseLand (L1): cover the hand's unmet colour
// needs, prefer basic, then the least flexible, then the earlier option. A
// land that would not stay is never picked, even when it is all that is
// offered.
func (bt *Bot) chooseLand(cs []cand) int {
	b := &bt.b
	anyLand := false
	for i := range cs {
		if cs[i].kind == dirac.AkPlayLand {
			anyLand = true
			break
		}
	}
	if !anyLand {
		return -1
	}
	unmet := b.unmetNeed()
	best := -1
	var bestCover int32 = -1
	bestBasic := false
	bestFlex := 0
	for i := range cs {
		if cs[i].kind != dirac.AkPlayLand {
			continue
		}
		c := b.cardOf(cs[i].obj)
		f := c.f
		if f == nil {
			f = &zeroCard
		}
		if f.EntryReturn != 0 && !b.s.ControlsMatching(b.me, f.EntryReturn) {
			continue
		}
		cover, basic, flex := landScore(f, c.basic, &unmet)
		if landBetter(best == -1, cover, basic, flex, bestCover, bestBasic, bestFlex) {
			best, bestCover, bestBasic, bestFlex = i, cover, basic, flex
		}
	}
	return best
}

// unmetNeed is chooseLand's unmet colour need: the castable own cards'
// pips minus what the own battlefield produces and the pool holds.
func (b *board) unmetNeed() [5]int32 {
	var need, avail [5]int32
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own {
			continue
		}
		if c.castable && c.cmc > 0 {
			for k := 0; k < 5; k++ {
				need[k] += int32(c.f.Pips[k])
			}
		}
		if c.onBF {
			for k := 0; k < 5; k++ {
				if c.f.Produces[k] > 0 {
					avail[k] += int32(c.f.Produces[k])
				}
			}
		}
	}
	for k := 0; k < 5; k++ {
		avail[k] += b.pool[k]
	}
	var unmet [5]int32
	for k := 0; k < 5; k++ {
		if u := need[k] - avail[k]; u > 0 {
			unmet[k] = u
		}
	}
	return unmet
}

// landScore is chooseLand's measure of a land f: the unmet pips it covers,
// whether it is basic, and how many colours it makes.
func landScore(f *CardFacts, basic bool, unmet *[5]int32) (cover int32, isBasic bool, flex int) {
	for k := 0; k < 5; k++ {
		if unmet[k] > 0 {
			take := int32(f.Produces[k])
			if take > unmet[k] {
				take = unmet[k]
			}
			cover += take
		}
	}
	return cover, basic, f.distinct()
}

// landBetter is chooseLand's order: more cover, then basic, then fewer
// colours; first is the first land seen.
func landBetter(first bool, cover int32, basic bool, flex int, bestCover int32, bestBasic bool, bestFlex int) bool {
	return first || cover > bestCover ||
		(cover == bestCover && basic && !bestBasic) ||
		(cover == bestCover && basic == bestBasic && flex < bestFlex)
}

// Default cast weights (botpolicy.DefaultCastWeights; the context and
// timing weights are 0, the hold threshold never fires).
const (
	wCreatureBase  = 30
	wCreaturePower = 4
	wNonCreature   = 1
	wKicked        = 6
	wFlashback     = 4
	wReserve       = 5
)

// cardWorth is botpolicy's cardWorth.
func (b *board) cardWorth(o dirac.Obj) int32 {
	c := b.cardOf(o)
	if c.creature {
		return wCreatureBase + wCreaturePower*c.power
	}
	return wNonCreature * c.cmc
}

// diesAtZeroX is cast.go's diesAtZeroX: a cast from a zone other than hand of a
// creature with no toughness of its own whose {X} the floating pool, which bounds
// the X ask, can't raise above 0. It would die at once (CR 704.5f), and from the
// graveyard the bot would repeat the cast forever.
func (b *board) diesAtZeroX(c *cand, cf *cardF, pool int32) bool {
	if cf.f == nil || c.mode == 0 && !c.offHand || !cf.f.Creature || !cf.f.NoToughness || !cf.f.HasX {
		return false
	}
	return pool < int32(cf.f.CMC)+int32(cf.f.PerX)
}

// chooseCast is cast.go's chooseCast with the default weights: worth plus
// the mode premium, plus the reserve bonus for a cast that keeps the
// cheapest instant-speed card's cost; a counter only at a foreign spell.
func (bt *Bot) chooseCast(me uint8, cs []cand) int {
	b := &bt.b
	anyCast := false
	for i := range cs {
		if cs[i].kind == dirac.AkCast {
			anyCast = true
			break
		}
	}
	if !anyCast {
		return -1
	}
	res := b.reserve()
	foreign := b.foreignSpell(me)
	poolTotal := b.poolTotal()
	producible := int32(-1)
	best := -1
	var bestScore int32 = -1
	for i := range cs {
		c := &cs[i]
		if c.kind != dirac.AkCast {
			continue
		}
		cf := b.cardOf(c.obj)
		if cf.counter && !foreign {
			continue
		}
		if bt.hold && b.instantHeld(me, cf) {
			continue
		}
		if b.diesAtZeroX(c, cf, poolTotal) {
			continue
		}
		s := b.cardWorth(c.obj)
		if c.mode&1 != 0 {
			s += wKicked
		} else if c.mode&2 != 0 {
			s += wFlashback
		}
		cost := cf.cmc
		pool := poolTotal
		if c.planned {
			if producible < 0 {
				producible = b.producible()
			}
			pool = producible
		}
		if res > 0 && pool-cost >= res {
			s += res * wReserve
		}
		if Debugf != nil {
			Debugf("cast obj %d worth %d mode %d cost %d pool %d res %d planned %v -> %d", c.obj, b.cardWorth(c.obj), c.mode, cost, pool, res, c.planned, s)
		}
		if best == -1 || s > bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// manaReaches is policy.go's manaReaches: the seat's unrestricted pool and its
// untapped sources can pay pips and generic. A source of several colours makes
// one of them per tap, so each set of colours needs that many sources; the pool
// pays first, and what it leaves pays generic.
func (b *board) manaReaches(pips [5]int32, generic int32) bool {
	spare := b.pool[5] - b.poolR[5]
	var short int32
	for k := range pips {
		pool := b.pool[k] - b.poolR[k]
		spare += max(pool-pips[k], 0)
		pips[k] = max(pips[k]-pool, 0)
		short += pips[k]
	}
	for set := 1; set < 1<<5; set++ {
		var need, have int32
		for k, n := range pips {
			if set&(1<<k) != 0 {
				need += n
			}
		}
		if need == 0 {
			continue
		}
		for i := 0; i < b.nCard; i++ {
			have += b.card[i].sourceMakes(set)
		}
		if have < need {
			return false
		}
	}
	var total int32
	for i := 0; i < b.nCard; i++ {
		total += b.card[i].sourceMakes(1<<6 - 1)
	}
	return total+spare >= short+generic
}

// sourceMakes is the most c makes per tap among the colours of set, 0 when it
// can't be tapped for mana now.
func (c *cardF) sourceMakes(set int) int32 {
	if !c.own || !c.onBF || c.tapped || c.creature && c.sick || c.f == nil {
		return 0
	}
	var most int32
	for k, n := range c.f.Produces {
		if set&(1<<k) != 0 {
			most = max(most, int32(n))
		}
	}
	return most
}

// reserve is cast.go's reserve: the cheapest castable instant-speed card's
// mana value (0 none).
func (b *board) reserve() int32 {
	var m int32 = -1
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.castable || !c.instant || c.cmc <= 0 {
			continue
		}
		if m == -1 || c.cmc < m {
			m = c.cmc
		}
	}
	if m < 0 {
		return 0
	}
	return m
}

// producible is cast.go's producibleMana: the pool plus every untapped own
// battlefield card's production.
func (b *board) producible() int32 {
	total := b.poolTotal()
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.onBF || c.tapped {
			continue
		}
		for k := 0; k < 6; k++ {
			total += int32(c.f.Produces[k])
		}
	}
	return total
}

// chooseAbility is ability.go's chooseAbility (A1-A6).
func (bt *Bot) chooseAbility(me uint8, cs []cand) int {
	best := -1
	var bestScore int32 = -1
	for i := range cs {
		c := &cs[i]
		if c.kind != dirac.AkActivate {
			continue
		}
		s, worth := bt.abilityScore(c.obj, c.mode, me)
		if !worth {
			continue
		}
		if best == -1 || s > bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

const maxActivationsPerTurn = 4

// abilityScore is ability.go's abilityScore.
func (bt *Bot) abilityScore(o dirac.Obj, ab uint8, me uint8) (int32, bool) {
	b := &bt.b
	s := b.s
	af := bt.f.ab(s.ObjCard(o), ab)
	if ab&0x80 != 0 {
		af = &zeroAb
	}
	if af.Attach {
		// A1 equipNoOp: no own creature to attach to, or already attached
		if !b.hasOwnCreature(me, dirac.NoObj) || b.cardOf(o).attached != dirac.NoObj {
			return 0, false
		}
	}
	if af.GrantOK && bt.grantNoOp(o, ab, af) {
		return 0, false
	}
	if af.Boon && !b.hasOwnCreature(me, o) {
		return 0, false // A1c
	}
	if b.cardOf(o).activated >= maxActivationsPerTurn {
		return 0, false // A5
	}
	if af.SkipTurns > 0 {
		return 0, false // A6: an own source never pays for a skipped turn
	}
	if af.Loyal {
		return bt.loyaltyScore(o, af, me), true
	}
	return 1000 - int32(af.Cost), true
}

// hasOwnCreature: a creature me controls other than except.
func (b *board) hasOwnCreature(me uint8, except dirac.Obj) bool {
	for i := 0; i < b.nCre; i++ {
		if b.cre[i].ctrl == me && b.cre[i].obj != except {
			return true
		}
	}
	return false
}

// grantNoOp is A1's redundant keyword grant: the source has every granted
// keyword already, or the same grant from it is on the stack.
func (bt *Bot) grantNoOp(o dirac.Obj, ab uint8, af *AbFacts) bool {
	s := bt.b.s
	if af.Grant != 0 {
		kw, _ := s.FeatKeywords(o)
		if kw&af.Grant == af.Grant {
			return true
		}
	}
	for i := 0; i < s.StackLen(); i++ {
		kind, src, card, _ := s.StackEntry(i)
		if kind != 1 || src != o {
			continue
		}
		sf := bt.f.ab(card, s.StackAbility(i))
		if sf.GrantOK && sf.Grant == af.Grant {
			return true
		}
	}
	return false
}

// loyaltyScore is ability.go's loyaltyScore.
func (bt *Bot) loyaltyScore(o dirac.Obj, af *AbFacts, me uint8) int32 {
	const (
		ultimate    = 1900
		removal     = 1500
		plus        = 1200
		zero        = 1150
		minus       = 1100
		minusX      = 1050
		killsWalker = 900
	)
	b := &bt.b
	dies := !af.LX && int32(b.s.Counters(o, kop.CtrLoyalty))+int32(af.LDelta) <= 0
	oppCre := false
	for i := 0; i < b.nCre; i++ {
		if b.cre[i].ctrl != me {
			oppCre = true
		}
	}
	switch {
	case af.LUltimate:
		return ultimate
	case af.LRemove && oppCre:
		if dies {
			return killsWalker + 100
		}
		return removal
	case af.LDelta > 0:
		return min(plus+int32(af.LDelta), removal-1)
	case af.LX:
		return minusX
	case dies:
		return killsWalker
	case af.LDelta == 0:
		return zero
	}
	return minus + int32(af.LDelta)
}

// wantsManual is heuristic.Bot.wantsManual: with no plan, any castable
// unplanned card worth the manual taps; with plans, the tap gate's own best
// intent when no plan pays it.
func (bt *Bot) wantsManual(s *dirac.State, d *dirac.Decision, np int) bool {
	b := &bt.b
	offered := bt.offeredColours(s, d.Player)
	if np == 0 {
		for i := 0; i < b.nCard; i++ {
			c := &b.card[i]
			if !c.own || !c.castable || c.cmc <= 0 || !bt.tapTarget(d, c) {
				continue
			}
			if bt.hold && b.instantHeld(d.Player, c) {
				continue
			}
			if b.poolPaysCard(c) || !b.offeredClosesPips(c, offered) {
				continue
			}
			if bt.castableNowCard(d.Player, c) {
				return true
			}
		}
		return false
	}
	c := bt.bestUnpayable(d, offered)
	if c == nil || !bt.castableNowCard(d.Player, c) {
		return false
	}
	for i := 0; i < np; i++ {
		if bt.payable[i] == c.obj {
			return false
		}
	}
	return true
}

// tapWould reports whether the manual policy's tap gate (chooseTap) taps
// now: some castable card the pool does not pay, and an unpaid card the
// offered sources can close.
func (bt *Bot) tapWould(s *dirac.State, d *dirac.Decision) bool {
	if s.FeatManaSources(d.Player) == 0 {
		return false
	}
	return bt.bestUnpayable(d, bt.offeredColours(s, d.Player)) != nil
}

// tapTarget is tap.go's tapTarget: c is among gorge's potential casts
// (castable now ignoring mana) and its planner did not prove it
// unpayable. The kernel offers exactly the casts some payment affords; a
// card gorge's planner never prices is potential whenever its timing
// allows (the planner reports it unsupported, not unpayable). A face cast
// says nothing: the plain cast it stands beside can still be proven unpayable.
// Under auto-pay gorge's census is the planned casts and the plain casts in
// hand its planner declines for their cost's shape
// (decision.PotentialShapeCasts); the kernel offers a planned cast, and a
// shape cast is one the planner declines for the card's or the board's
// shape (FeatPlanDeclinesShape: a convoke cast too, The Wandering Rescuer).
func (bt *Bot) tapTarget(d *dirac.Decision, c *cardF) bool {
	b := &bt.b
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == dirac.AkCast && dirac.Obj(a.Source()) == c.obj && a.Mode()&dirac.FeatCastFace == 0 {
			return true
		}
	}
	if b.s.ObjZone(c.obj) != kop.ZoneHand || !b.s.FeatPlanDeclinesShape(b.me, c.obj) || !(c.instant || b.myTur && b.isMain && b.nStack == 0) {
		return false
	}
	return bt.potential(c)
}

// potential is tap.go's census reading of a shape cast (decision.PotentialCasts):
// gorge's potential walk prices the card against PotentialMana, each untapped
// source counted once for every colour it makes (a source of any colour, no
// colour listed, for each), so a coloured pip the sources cannot make (Eaten
// Alive's {B} beside a Forest and Thornwood Falls, Goblin Negotiation's {R}{R}
// on one Swiftwater Cliffs) or a mana value beyond their sum keeps it out
// however much is floated. An either-or additional cost is priced as the walk
// prices it (FeatAltAddMana): Eaten Alive with no creature to sacrifice costs
// {3}{B}{B}, not {B}.
func (bt *Bot) potential(c *cardF) bool {
	b := &bt.b
	part, ok := b.s.FeatAltAddMana(b.me, c.obj)
	if !ok {
		return false
	}
	var avail [6]int32
	for k := 0; k < 6; k++ {
		avail[k] = b.pool[k]
	}
	for i := 0; i < b.nCard; i++ {
		x := &b.card[i]
		if !x.own || !x.onBF || x.tapped || !b.s.FeatManaActivatable(x.obj) {
			continue
		}
		pf := x.f
		anyCol := pf.ProdReflected || pf.ProdAny && pf.Produces[0] <= 0 && pf.Produces[1] <= 0 && pf.Produces[2] <= 0 && pf.Produces[3] <= 0 && pf.Produces[4] <= 0
		for k := 0; k < 6; k++ {
			switch {
			case pf.Produces[k] > 0:
				avail[k] += int32(pf.Produces[k])
			case anyCol && k < 5:
				avail[k]++
			}
		}
	}
	var total int32
	for k := 0; k < 6; k++ {
		total += avail[k]
		if k < 5 && int32(c.f.Pips[k])+int32(part.Pips[k]) > avail[k] {
			return false
		}
	}
	return c.cmc+int32(part.MV()) <= total
}

// poolPaysCard is tap.go's poolPays.
func (b *board) poolPaysCard(c *cardF) bool {
	if b.poolTotal() < c.cmc {
		return false
	}
	for k := 0; k < 5; k++ {
		if int32(c.f.Pips[k]) > b.pool[k] {
			return false
		}
	}
	return true
}

// offeredColours is tap.go's offeredColours over the seat's manual mana
// sources (each "activate" option's card production).
func (bt *Bot) offeredColours(s *dirac.State, me uint8) [5]bool {
	var off [5]bool
	b := &bt.b
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.onBF || c.tapped {
			continue
		}
		pf := c.f
		if !pf.ProdReflected && !pf.ProdAny && pf.Produces[0] <= 0 && pf.Produces[1] <= 0 && pf.Produces[2] <= 0 && pf.Produces[3] <= 0 && pf.Produces[4] <= 0 {
			continue // colours nothing: off is unchanged either way
		}
		if !s.FeatManaActivatable(c.obj) {
			continue
		}
		for k := 0; k < 5; k++ {
			if c.f.Produces[k] > 0 || c.f.ProdReflected || c.f.ProdAny {
				off[k] = true
			}
		}
	}
	return off
}

// offeredClosesPips is tap.go's offeredClosesPips.
func (b *board) offeredClosesPips(c *cardF, off [5]bool) bool {
	for k := 0; k < 5; k++ {
		if int32(c.f.Pips[k]) > b.pool[k] && !off[k] {
			return false
		}
	}
	return true
}

// bestUnpayable is tap.go's bestUnpayable (cardScore, ties to the lower
// object id).
func (bt *Bot) bestUnpayable(d *dirac.Decision, off [5]bool) *cardF {
	return bt.bestUnpayableOf(d, off, true)
}

// bestUnpayableOf is bestUnpayable; a source's colour ask carries no list of
// potential casts, so offered is false there and tapTarget reads nothing.
func (bt *Bot) bestUnpayableOf(d *dirac.Decision, off [5]bool, offered bool) *cardF {
	b := &bt.b
	var best *cardF
	var bestScore int32 = -1
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if !c.own || !c.castable || c.cmc <= 0 || offered && !bt.tapTarget(d, c) {
			continue
		}
		if bt.hold && b.instantHeld(d.Player, c) {
			continue
		}
		if b.poolPaysCard(c) || !b.offeredClosesPips(c, off) {
			continue
		}
		s := c.cmc
		if c.creature {
			s = 30 + c.power*4
		}
		if best == nil || s > bestScore || (s == bestScore && c.obj < best.obj) {
			best, bestScore = c, s
		}
	}
	return best
}

// castableNowCard is tap.go's castableNowCard.
func (bt *Bot) castableNowCard(me uint8, c *cardF) bool {
	b := &bt.b
	if !c.instant && !(b.myTur && b.isMain && b.nStack == 0) {
		return false
	}
	if c.counter && !b.foreignSpell(me) {
		return false
	}
	if c.cmc > b.producible() && !b.indeterminateUntapped() {
		return false
	}
	return true
}

// indeterminateUntapped is tap.go's indeterminateSourceUntapped: an
// untapped own battlefield source whose amount the production cannot
// price.
func (b *board) indeterminateUntapped() bool {
	for i := 0; i < b.nCard; i++ {
		c := &b.card[i]
		if c.own && c.onBF && !c.tapped && c.f.ProdIndet {
			return true
		}
	}
	return false
}
