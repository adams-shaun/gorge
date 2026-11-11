package feat

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// listpos.go places each kernel option where gorge's network sees it: the
// option index/count feature (dense[odIndex] = idx/(n-1)) is computed over
// the list gorge's net scores, not over the kernel's canonical list. That
// list is gorge's decision.Options -- each kind in its engine's enumeration
// order -- and, at a priority decision, the manual mana-tap "activate"
// options and "concede" the kernel never offers, with one synthetic cast per
// planner-paid cast that has no legacy cast option appended after them
// (azmcts priorsWith's payCasts form: the bot's first, then engine order).
//
// Each option gets a sort key reproducing gorge's order among the kernel's
// options; its position is its rank plus the gorge-only entries before it.

// noPos marks an option gorge's list has no entry for: Done (gorge poses a
// multi-pick answer as one decision), and a cast the floating pool cannot
// pay that gorge's planner offers no payment action for -- a kicked cast
// (it plans plain and flashback casts only), or an {X} spell, one with an
// either-or additional cost or a target-conditional cost reduction, or one
// payable only with a granted mana ability (dirac.State.FeatPlannedCast); gorge reaches those by lowering a
// potential play and its search never scores them. Such an option is encoded as the list's last
// entry.
const noPos = ^uint32(0)

// Priority sections, in gorge's walk order (engine/rules legal.go: the hand
// walk, graveyard casts, the battlefield walk's mana section then its
// ability section, pass, concede), then the appended synthetic casts.
const (
	secHand     = 0 // land plays and pool-paid casts, per hand card
	secGrave    = 1 // pool-paid flashback casts
	secAbility  = 3 // after the mana-tap section
	secPass     = 4
	secSynthBot = 6 // after concede: the bot's synthetic cast
	secSynth    = 7
)

// zoneKey is obj's place in gorge's zone lists: zone class, controller (or
// owner) seat and list index, packed so a smaller key is earlier; the stack
// sorts first, bottom entry first.
type zoneKey struct {
	zone, seat uint8
	idx        uint16
	ok         bool
}

func (z zoneKey) key() uint32 {
	return uint32(z.zone)<<20 | uint32(z.seat)<<16 | uint32(z.idx)
}

// zoneOf places obj in gorge's zone lists: the stack (bottom first), a
// controller's battlefield (the view's entry order), and an owner's
// graveyard, exile, hand or library (the projection's zone index; the
// library from the top). An opponent's hand is placed too (a revealed-hand
// discard choice).
func (e *Encoder) zoneOf(o uint8) zoneKey {
	if o == noObj || int(o) >= int(e.kp.NObj) {
		return zoneKey{}
	}
	ob := e.po(o)
	v := &e.v
	switch ob.Zone {
	case kop.ZoneStack:
		for i := range v.stack {
			if ci := v.stack[i].cv; ci >= 0 && v.cards[ci].obj == o {
				return zoneKey{zone: 0, idx: uint16(i), ok: true}
			}
		}
	case kop.ZoneBattlefield:
		for i, ci := range v.players[ob.Ctrl&1].bf {
			if v.cards[ci].obj == o {
				return zoneKey{zone: 1, seat: ob.Ctrl & 1, idx: uint16(i), ok: true}
			}
		}
	case kop.ZoneGraveyard:
		return zoneKey{zone: 2, seat: ob.Owner & 1, idx: uint16(e.st0.FeatZoneIndex(dirac.Obj(o))), ok: true}
	case kop.ZoneExile:
		return zoneKey{zone: 3, seat: ob.Owner & 1, idx: uint16(e.st0.FeatZoneIndex(dirac.Obj(o))), ok: true}
	case kop.ZoneHand:
		return zoneKey{zone: 4, seat: ob.Owner & 1, idx: uint16(e.st0.FeatZoneIndex(dirac.Obj(o))), ok: true}
	case kop.ZoneLibrary:
		return zoneKey{zone: 5, seat: ob.Owner & 1, idx: uint16(e.st0.FeatZoneIndex(dirac.Obj(o))), ok: true}
	}
	return zoneKey{}
}

// optKey is option a's gorge-order key at decision d (noPos: none).
func (e *Encoder) optKey(d *dirac.Decision, a dirac.Action) uint32 {
	if a.Kind() == dirac.AkDone || d.Kind == dirac.DkTarget && a.Target() == dirac.NoTarget {
		return noPos // declining an "up to one" target is gorge's done
	}
	src, tgt, mode := a.Source(), a.Target(), a.Mode()
	switch d.Kind {
	case dirac.DkPriority:
		switch a.Kind() {
		case dirac.AkPass:
			return secPass << 24
		case dirac.AkPlayLand:
			// the hand walk, then a land played from another zone through a
			// permission (mayPlayLandWalk)
			z := e.zoneOf(src)
			if z.zone != 4 {
				return secHand<<24 | 1<<20 | uint32(z.idx)<<8
			}
			return secHand<<24 | uint32(z.idx)<<8
		case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire:
			if e.st0.FeatScripted(d.Player, a) {
				return noPos // a scripted potential play (Bloom Tender's prefix)
			}
			z := e.zoneOf(src)
			sub := uint32(0) // the hand walk
			if mode&2 == 0 && z.zone != 4 {
				sub = 2 // a spell cast from another zone through a permission (mayPlaySpellWalk)
			}
			if e.st0.FeatPoolPays(d.Player, dirac.Obj(src), mode) {
				if mode&2 != 0 {
					return secGrave<<24 | uint32(z.idx)<<8
				}
				return secHand<<24 | sub<<20 | uint32(z.idx)<<8 | 1 + uint32(mode&1)
			}
			if mode&dirac.FeatCastFree != 0 && mode&2 == 0 && z.zone == 4 &&
				e.st0.FeatMayPlayUntyped(d.Player, dirac.Obj(src)) {
				// a cost-waiving permission's cast from the hand (CR 118.9):
				// gorge's hand walk still offers the printed arm -- its cost
				// is the free alternative -- so it ranks like a pool-paid
				// cast, before the pass.
				return secHand<<24 | uint32(z.idx)<<8 | 1 + uint32(mode&1)
			}
			if mode&1 != 0 || !e.st0.FeatPlannedCast(d.Player, dirac.Obj(src), mode) {
				return noPos
			}
			if mode&dirac.FeatCastMayPlay != 0 && !e.st0.FeatMayPlayUntyped(d.Player, dirac.Obj(src)) {
				return noPos // a typed permission's cast: lowered, not in gorge's list
			}
			// engine order: the hand walk, the permission walks, then
			// graveyard casts
			if mode&2 != 0 {
				sub = 3
			}
			return secSynth<<24 | sub<<16 | uint32(z.idx)<<8
		case dirac.AkActivate:
			if e.st0.FeatScripted(d.Player, a) {
				return noPos
			}
			if !e.st0.FeatPlannedActivation(d.Player, dirac.Obj(src), mode) && !e.st0.FeatAbilityPoolPays(d.Player, dirac.Obj(src), mode, dirac.Obj(tgt)) {
				return noPos // reached only by a scripted potential play
			}
			// the battlefield walk's zones: battlefield, stack, graveyard,
			// hand, exile; each seat's list in order; ability index
			z := e.zoneOf(src)
			if mode&0x80 != 0 {
				// a granted activation (CR 613.1f): gorge's walk offers the
				// granted abilities in a sweep after every printed ability,
				// seat by seat then host in battlefield order, so key above
				// every printed slot, ordered by the host's own position.
				return secAbility<<24 | 6<<20 | uint32(z.seat)<<16 | uint32(z.idx)<<8 | uint32(mode)
			}
			zr := [6]uint32{1, 0, 2, 4, 3, 5}[min(int(z.zone), 5)] // zoneKey: 0 stack, 1 bf, 2 gy, 3 exile, 4 hand
			return secAbility<<24 | zr<<20 | uint32(z.seat)<<16 | uint32(z.idx)<<8 | uint32(mode)
		}
		return noPos
	case dirac.DkAttackers:
		// defender-major: each defending player, then the attacking
		// creatures in battlefield order, each player pair followed by its
		// planeswalker pairs.
		c := uint32(e.zoneOf(src).idx)
		if tgt >= dirac.PlayerTarget0 {
			return uint32(tgt-dirac.PlayerTarget0)<<24 | c<<12
		}
		w := e.zoneOf(tgt)
		return uint32(w.seat)<<24 | c<<12 | 1 + uint32(w.idx)
	case dirac.DkBlockers:
		// each blocker in its controller's battlefield order, then each
		// attacker in the active player's.
		return uint32(e.zoneOf(src).idx)<<12 | uint32(e.zoneOf(tgt).idx)
	case dirac.DkTarget:
		var copyAsk uint32
		if d.Sub == dirac.SubCopyTarget {
			// a spell copy's new-target ask: gorge lists the inherited
			// targets first, then the census in its usual order
			if i := e.st0.FeatCopyInherited(tgt); i >= 0 {
				return uint32(i)
			}
			copyAsk = 1 << 30
		}
		// players first, then objects zone by zone, seat by seat.
		if tgt >= dirac.PlayerTarget0 {
			return copyAsk | uint32(tgt-dirac.PlayerTarget0)
		}
		return copyAsk | 1<<24 | e.zoneOf(tgt).key()
	case dirac.DkModes:
		return uint32(mode)
	case dirac.DkTriggerOrder:
		// gorge lists the pending triggers in detection order -- event by
		// event, each event's in its battlefield walk -- which is the
		// kernel's queue order (it walks listeners the same way)
		return uint32(tgt)
	case dirac.DkMulligan:
		return uint32(a.Kind())
	case dirac.DkChoose:
		switch d.Sub {
		case dirac.ChOptional, dirac.ChWardPay, dirac.ChOptDiscMay:
			return 1 - uint32(mode&1) // yes, then no
		case dirac.ChGenMayShuffle:
			return 1 - uint32(mode&1) // yes, then no
		case dirac.ChGenDivide, dirac.ChCastDivide:
			return uint32(e.st0.FeatDivideIndex(src)) // gorge lists the targets in target order
		case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick, dirac.ChDivide, dirac.ChGenBranch, dirac.ChGenPileChoose, dirac.ChGenColor, dirac.ChAltAdd, dirac.ChGenPip, dirac.ChGenGift, dirac.ChCostForage, dirac.ChGenKeepTapped:
			return uint32(mode) // ChAltAdd: the payable parts first, as gorge lists them
		case dirac.ChGenPay, dirac.ChGenPayMode, dirac.ChGenTopBottom, dirac.ChGenLeyline:
			return 1 - uint32(mode&1) // pay / top / yes first
		case dirac.ChGenType:
			return e.st0.FeatTypeKey(d.Player, int(tgt)<<8|int(mode)) // the chooser's own creature types first
		case dirac.ChGenChooseCard, dirac.ChGenChooseEach:
			return uint32(src) // gorge's ChooseCard walks the objects in id order (effects cardChoices)
		case dirac.ChGenCastFree:
			return uint32(src) // gorge's modes over the castable cards, in object order
		case dirac.ChGenReplOrder:
			return uint32(mode) // the candidate's index in gorge's replacement scan
		case dirac.ChCommanderZone:
			return uint32(mode) // command_zone (0) then leave (1), as gorge lists them
		}
		return e.zoneOf(src).key()
	}
	return uint32(a)
}

// listShape computes, for every option of d, its position in gorge's net
// list (pos) and the list's length. bot is the index in d of the bot's
// answer (-1 none): a synthetic cast it names is listed first among them.
func (e *Encoder) listShape(d *dirac.Decision, bot int) int {
	n := int(d.N)
	keys := &e.opt.keys
	extra := 0 // gorge-only entries: the mana taps
	if d.Kind == dirac.DkPriority {
		// kernel FeatManaSources, read from the view
		for _, ci := range e.v.players[d.Player&1].bf {
			if e.v.cards[ci].manaSrc {
				extra++
			}
		}
	}
	have := 0
	for i := 0; i < n; i++ {
		k := e.optKey(d, d.Options[i])
		if i == bot && k>>24 == secSynth && d.Kind == dirac.DkPriority {
			k = secSynthBot << 24
		}
		keys[i] = k
		if k != noPos {
			have++
		}
	}
	total := have
	if d.Kind == dirac.DkPriority {
		total += extra + 1 // + concede
	}
	if d.Kind == dirac.DkChoose && d.Sub == dirac.ChGenPay && e.st0.FeatPayManual() {
		total++ // gorge's manual pay route, listed last
	}
	for i := 0; i < n; i++ {
		k := keys[i]
		if k == noPos {
			e.opt.pos[i] = int16(total - 1)
			continue
		}
		r := 0
		for j := 0; j < n; j++ {
			if keys[j] < k {
				r++
			}
		}
		if d.Kind == dirac.DkPriority {
			if k>>24 >= secAbility {
				r += extra
			}
			if k>>24 >= secSynthBot {
				r++ // concede
			}
		}
		e.opt.pos[i] = int16(r)
	}
	return total
}
