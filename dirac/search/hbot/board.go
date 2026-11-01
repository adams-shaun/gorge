package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// board is botpolicy.Board read off a kernel State for the deciding seat:
// fixed arrays, refilled per decision, no allocation. Its tables keep
// gorge's insertion orders (botpolicy IDTable iterates in insertion order):
// creatures seat 0's battlefield then seat 1's, each in entry order; cards
// the seat's hand, graveyard, battlefield, then the other seats'
// battlefields.
type board struct {
	s  *dirac.State
	f  *Facts
	me uint8

	step                     kop.Step
	isMain, firstMain, myTur bool
	life                     [2]int32
	pool                     [6]int32 // W U B R G C
	poolR                    [6]int32 // the restricted part
	libSize, handSize        int32

	nCre int
	cre  [dirac.MaxPerm * 2]creature
	creX [dirac.MaxObj]int16 // obj -> 1 + index into cre, 0 none

	nCard int
	card  [dirac.MaxObj]cardF
	cardX [dirac.MaxObj]int16 // obj -> 1 + index into card, 0 none

	nStack int
	stack  [dirac.MaxStack]stackE

	objs [dirac.MaxPerm]dirac.Obj // scratch

	// bf holds each seat's battlefield list (entry order) for the duration
	// of one fill, so the creature census and the card census read one
	// FeatBattlefield walk per seat, not two.
	bf  [2][dirac.MaxPerm]dirac.Obj
	nbf [2]int

	// d is fillPriority's decision (staticsIdle reads its cast options).
	d *dirac.Decision
	// creOnly: fillCombat's creature-only census.
	creOnly bool
	// forceFull makes fillPriority and fillCombat read the whole board
	// (fill): the reference TestLiteFillsMatchFull compares against.
	forceFull bool

	// bfKw is the union of ContKw over both battlefields' cards (every
	// keyword a Continuous static in force could grant), when
	// Facts.contKw; all bits otherwise.
	bfKw kop.KwMask

	// bfPos is a battlefield object's index in its controller's entry
	// order (gorge's battlefield zone list).
	bfPos [dirac.MaxObj]uint8

	// cs is combat.go's scratch (attack and block declarations).
	cs combatScratch
}

// creature is botpolicy.Creature.
type creature struct {
	obj                  dirac.Obj
	power, tough, damage int32
	kw                   kop.KwMask
	tapped               bool
	ctrl                 uint8
	attacking, blocking  bool
	sick                 bool
	cardID               dirac.CardID
}

// cardF is botpolicy.Card.
type cardF struct {
	obj          dirac.Obj
	f            *CardFacts
	creature     bool
	power, tough int32
	cmc          int32
	basic        bool
	attached     dirac.Obj
	activated    int32
	castable     bool
	onBF         bool
	instant      bool
	tapped, sick bool
	counter      bool
	own          bool // a seat-relative entry (the deciding seat's own zones)
}

// stackE is botpolicy.StackEntry.
type stackE struct {
	obj     dirac.Obj
	ctrl    uint8
	isSpell bool
	cmc     int32
	card    dirac.CardID // the spell's card (its printed cost), NoCard for an ability
}

var noCardF cardF

// fill reads s for seat me.
func (b *board) fill(s *dirac.State, f *Facts, me uint8) { b.fillAs(s, f, me, true) }

// fillPriority is fill for the priority policy (priority.go, hold.go),
// which never reads a battlefield object's power, toughness, keywords or
// damage: those fields are left zero, skipping the derived P/T and keyword
// walks (each a pass over every static in force).
func (b *board) fillPriority(s *dirac.State, f *Facts, me uint8, d *dirac.Decision) {
	b.d = d
	b.fillAs(s, f, me, b.forceFull)
	b.d = nil
}

// fillCombat is fill for combat.go (attackers, blockers, Declaration),
// which reads only the creature census and the life totals: the card
// census (hand, graveyard, battlefield cards) is left empty.
func (b *board) fillCombat(s *dirac.State, f *Facts, me uint8) {
	b.creOnly = !b.forceFull
	b.fillAs(s, f, me, true)
	b.creOnly = false
}

// fillAs is fill; full=false leaves the battlefield P/T, keywords and
// damage zero (fillPriority).
func (b *board) fillAs(s *dirac.State, f *Facts, me uint8, full bool) {
	b.s, b.f, b.me = s, f, me
	b.step = s.StepNow()
	b.isMain = b.step == kop.StepMain1 || b.step == kop.StepMain2
	b.firstMain = b.step == kop.StepMain1
	b.myTur = s.ActivePlayer() == me
	for p := uint8(0); p < 2; p++ {
		b.life[p] = int32(s.LifeOf(p))
	}
	pool, rc, rn := s.PoolOf(me)
	for i := 0; i < 6; i++ {
		b.pool[i] = int32(pool[i])
		b.poolR[i] = 0
	}
	if rn != 0 && rc < 6 {
		b.poolR[rc] = int32(rn)
	}
	b.libSize = int32(s.LibrarySize(me))
	b.handSize = int32(s.HandSize(me))

	// the stack census, bottom to top
	b.nStack = 0
	for i := 0; i < s.StackLen() && i < len(b.stack); i++ {
		kind, src, card, ctrl := s.StackEntry(i)
		e := stackE{obj: src, ctrl: ctrl, isSpell: kind == 0, card: kop.NoCard}
		if kind == 0 {
			e.card = card
			e.cmc = int32(f.card(card).CMC)
		}
		b.stack[b.nStack] = e
		b.nStack++
	}

	// creatures: every seat's battlefield, in seat then entry order
	nobj := s.NumObjs()
	for i := 0; i < nobj; i++ {
		b.creX[i] = 0
		b.cardX[i] = 0
	}
	b.nCre = 0
	b.nCard = 0
	b.bfKw = 0
	if !f.contKw {
		b.bfKw = ^kop.KwMask(0)
	}
	for p := uint8(0); p < 2; p++ {
		objs := s.FeatBattlefield(p, b.bf[p][:0])
		b.nbf[p] = len(objs)
		for i, o := range objs {
			b.bfPos[o] = uint8(i)
			cid := s.ObjCard(o)
			cf := f.card(cid)
			b.bfKw |= cf.ContKw
			// combat.go's census reads the effective type words: an animated
			// land or a crewed Vehicle is in it, a creature that lost the type
			// is not. A face-down permanent is a 2/2 creature (CR 708.2).
			if !s.IsCreature(o) {
				continue
			}
			fl := s.HBotPermFlags(o)
			c := creature{obj: o, tapped: fl&dirac.ProjTapped != 0, ctrl: p,
				attacking: fl&dirac.ProjAttacking != 0, blocking: fl&dirac.ProjBlocking != 0,
				sick: fl&dirac.ProjSick != 0, cardID: cid}
			if full {
				pw, tg := s.PowerToughness(o)
				c.power, c.tough, c.damage = int32(int16(pw)), int32(int16(tg)), int32(s.Damage(o)) // the projection stores int16
				c.kw, _ = s.FeatKeywords(o)
			}
			b.cre[b.nCre] = c
			b.nCre++
			b.creX[o] = int16(b.nCre)
		}
	}
	if b.creOnly {
		return
	}

	// the seat's own cards: hand, graveyard, battlefield
	// a spell being cast is on gorge's stack, not in the hand census
	building := dirac.NoObj
	if _, _, _, ok := s.Building(); ok {
		building = s.FeatSource()
	}
	objs := s.FeatZoneList(me, kop.ZoneHand, b.objs[:0])
	for _, o := range objs {
		if o == building {
			continue
		}
		b.addOwn(o, kop.ZoneHand, full)
	}
	objs = s.FeatZoneList(me, kop.ZoneGraveyard, b.objs[:0])
	for _, o := range objs {
		b.addOwn(o, kop.ZoneGraveyard, full)
	}
	for _, o := range b.bf[me][:b.nbf[me]] {
		b.addOwn(o, kop.ZoneBattlefield, full)
	}
	// the other seat's battlefield: worth facts only
	for _, o := range b.bf[1-me][:b.nbf[1-me]] {
		cid := s.ObjCard(o)
		cf := f.card(cid)
		c := cardF{obj: o, f: cf, creature: b.creX[o] > 0, cmc: int32(cf.CMC), basic: cf.Basic,
			activated: int32(s.FeatActivations(o)), attached: dirac.NoObj}
		if i := b.creX[o]; i > 0 {
			c.power, c.tough = b.cre[i-1].power, b.cre[i-1].tough
		}
		b.card[b.nCard] = c
		b.nCard++
		b.cardX[o] = int16(b.nCard)
	}
}

// addOwn adds the seat's own object o (in zone z) to the card census.
func (b *board) addOwn(o dirac.Obj, z kop.Zone, full bool) {
	s, f := b.s, b.f
	cid := s.ObjCard(o)
	cf := f.card(cid)
	c := cardF{obj: o, f: cf, creature: cf.Creature, cmc: int32(cf.CMC), basic: cf.Basic, counter: cf.Counter,
		activated: int32(s.FeatActivations(o)), attached: dirac.NoObj, own: true}
	tbl := f.Tbl
	if z == kop.ZoneBattlefield {
		c.onBF = true
		_, att, _ := s.FeatObjLinks(o)
		c.attached = att
		// Only a creature's P/T is read from a battlefield card (cardWorth
		// and bestUnpayable gate on c.creature), and instant is never read
		// off one (castable stays false), so skip the keyword and P/T
		// walks for non-creature permanents.
		c.creature = b.creX[o] > 0
		if i := b.creX[o]; i > 0 {
			cr := &b.cre[i-1]
			c.power, c.tough = cr.power, cr.tough
			c.sick = cr.sick
			c.tapped = cr.tapped
		} else {
			c.tapped = s.Tapped(o)
		}
	} else {
		var kw kop.KwMask
		if !full && b.staticsIdle(o, cid, cf, z) {
			// fillPriority: nothing the priority policy reads can
			// change (staticsIdle); the P/T and instant stay zero, as
			// that policy never reads them here
			c.castable = z == kop.ZoneHand
			c.instant = cf.Instant || tbl.Cards[cid].Keywords&kop.KwFlash != 0
			b.card[b.nCard] = c
			b.nCard++
			b.cardX[o] = int16(b.nCard)
			return
		}
		if int(cid) < len(tbl.Cards) {
			cd := &tbl.Cards[cid]
			c.power, c.tough = int32(cd.Power), int32(cd.Toughness)
			kw = cd.Keywords
		}
		if cf.CDA {
			if p, t, ok := s.FeatCDAPT(o); ok {
				c.power, c.tough = int32(p), int32(t)
			}
		}
		// gorge's derived characteristics apply the battlefield's
		// Continuous statics to an off-battlefield card too
		dp, dt, skw, _ := s.FeatStaticPT(o)
		c.power += int32(dp)
		c.tough += int32(dt)
		kw |= skw
		c.castable = z == kop.ZoneHand || (z == kop.ZoneGraveyard && kw&kop.KwFlashback != 0)
		c.instant = cf.Instant || kw&kop.KwFlash != 0
	}
	b.card[b.nCard] = c
	b.nCard++
	b.cardX[o] = int16(b.nCard)
}

// staticsIdle reports, for fillPriority, that an off-battlefield own card
// o (cid, in zone z) needs no FeatStaticPT walk. The walk can change the
// census only through its keywords (flash: instant; flashback: a graveyard
// card's castable) and its P/T, and the priority policy reads a card's P/T
// only through c.creature-gated scores (cardWorth of a cast option's
// source, bestUnpayable over castable cards). So: no Continuous static on
// the battlefield grants flash or flashback (bfKw) or the card prints
// flashback, and the card is a noncreature (hand) or a graveyard card no
// cast option names (not castable, and no cast candidate scores it).
func (b *board) staticsIdle(o dirac.Obj, cid dirac.CardID, cf *CardFacts, z kop.Zone) bool {
	tbl := b.f.Tbl
	if tbl == nil || int(cid) >= len(tbl.Cards) {
		return false
	}
	if b.bfKw&kop.KwFlash != 0 {
		return false
	}
	switch z {
	case kop.ZoneHand:
		return !cf.Creature
	case kop.ZoneGraveyard:
		if (tbl.Cards[cid].Keywords|b.bfKw)&kop.KwFlashback != 0 {
			return false
		}
		if cf.Creature {
			d := b.d
			if d == nil {
				return false
			}
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; a.Kind() == dirac.AkCast && dirac.Obj(a.Source()) == o {
					return false
				}
			}
		}
		return true
	}
	return false
}

// cardOf is obj's census entry (zero facts when the census has none).
func (b *board) cardOf(o dirac.Obj) *cardF {
	if o != dirac.NoObj {
		if i := b.cardX[o]; i > 0 {
			return &b.card[i-1]
		}
	}
	return &noCardF
}

// creOf is obj's creature entry, nil when it is none.
func (b *board) creOf(o dirac.Obj) *creature {
	if o != dirac.NoObj {
		if i := b.creX[o]; i > 0 {
			return &b.cre[i-1]
		}
	}
	return nil
}

// poolTotal is Pool.Total().
func (b *board) poolTotal() int32 {
	var n int32
	for _, x := range b.pool {
		n += x
	}
	return n
}

// foreignSpell is Board.ForeignSpell(me).
func (b *board) foreignSpell(p uint8) bool {
	for i := 0; i < b.nStack; i++ {
		if b.stack[i].isSpell && b.stack[i].ctrl != p {
			return true
		}
	}
	return false
}

// remTough / pt / threat are Creature's.
func (c *creature) remTough() int32 { return c.tough - c.damage }
func (c *creature) pt() int32       { return c.power + c.tough }

func (c *creature) has(kw kop.KwMask) bool { return c.kw&kw != 0 }
