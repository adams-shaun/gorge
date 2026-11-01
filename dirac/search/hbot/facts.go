package hbot

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Facts is the per-table card knowledge the bot reads beyond the kernel's
// own table: the facts gorge's bot reads off strings (a card's printed
// cost text, its mana production, its spell API, an ability's label, the
// target effect of a targeted ability). It is plain data -- numbers and
// masks, no strings -- built once per table (dirac/oracle.HBotFacts reads
// them from gorge's compiled IR) and read-only afterwards, so any number of
// bots may share it. A table without Facts (NewFacts) falls back to what
// the kop table says.
type Facts struct {
	Tbl   *kop.Table
	Cards []CardFacts // per CardID
	// contKw reports that every CardFacts.ContKw is filled (NewFacts): a
	// hand-built Facts without it never lets the board skip a static walk.
	contKw bool
}

// CardFacts is one card's facts (botpolicy.Card's printed half).
type CardFacts struct {
	CMC         int8    // printed mana value (botpolicy.CmcOf)
	Pips        [5]int8 // single-colour W U B R G pips of the printed cost (colourPips)
	HasX        bool    // the printed cost has {X}
	PerX        int8    // the number of {X} symbols
	NoToughness bool    // the printed face's toughness is at most 0
	Creature    bool    // the printed face is a creature (Face.IsCreature)
	Basic       bool    // the type line has Basic
	Instant     bool    // the type line has Instant
	Counter     bool    // the SP$ line is a Counter (isCounterSpell)
	// Trick is the spell ability is a combat trick (gorge
	// effects.CastCombatTrick). NewFacts leaves it false: the kop table
	// carries no trick field, so clause (c) of the hold rule never fires
	// on a NewFacts table.
	Trick bool
	// FaceDmg is the literal damage of a "damage" removal spell whose
	// scope names players (gorge Board.lethalFace), 0 none. NewFacts
	// leaves it 0: the kop table carries no removal field, so the lethal
	// exception never fires on a NewFacts table.
	FaceDmg int16
	CDA     bool // the card has a StatCDAPT ability (FeatCDAPT can hit)
	// ContKw is every keyword the card's Continuous statics grant (the
	// union of their Static.Kw): kernel State.FeatStaticPT can add a
	// keyword to an off-battlefield card only when some battlefield
	// permanent's card has it here (emblems grant none there).
	ContKw kop.KwMask
	// Produces is the face's cards.ManaProduction: per-colour amounts
	// W U B R G C, any colour, reflected, indeterminate.
	Produces  [6]int8
	MakesMana bool // a mana ability is printed (not ManaProduction.IsZero)
	// FirstMana is the first colour (W U B R G C = 0..5) the printed mana
	// abilities offer, -1 none: what a tap gives the bot, which takes first offers.
	FirstMana int8
	// PlainMana is the colours (W U B R G = bits 0..4) a mana ability costing no more
	// than its {T} makes: the options of a colour ask that name a colour outright.
	PlainMana uint8
	// Converter: every mana ability spends mana without tapping its source
	// (botpolicy converterCost).
	Converter     bool
	ProdAny       bool
	ProdReflected bool
	ProdIndet     bool
	// EntryReturn is the filter of the land the card's enters trigger makes its
	// controller return to hand, 0 none: with no land matching it when played, the
	// land returns itself or is sacrificed (gorge Option.LandWontStay).
	EntryReturn kop.FilterID
	Abilities   []AbFacts // per kop ability index of the card
	// SpellTgt is the spell's targeted SAs (the card's spell ability chain).
	SpellTgt []TgtEff
}

// AbFacts is one kop ability's facts as gorge's priority option and target
// ask carry them.
type AbFacts struct {
	Cost   int16 // botpolicy abilityCost of the option label (first digit run, else 20)
	Attach bool  // an AB$ Attach (Equip)
	// Grant is the keyword mask of a pure keyword grant (decision.Grant
	// non-nil); GrantOK reports the ability is one.
	Grant   kop.KwMask
	GrantOK bool
	Boon    bool // GrantStatics are all one-way boons (boonStaticsOnly)
	// Loyalty facts (decision.Loyalty), Loyal when it is a loyalty ability.
	Loyal                  bool
	LDelta                 int8
	LX, LUltimate, LRemove bool
	SkipTurns              int8 // SelfSkipTurns
	TapCost                bool // its cost taps the source
	// Tgt is the ability's targeted SAs in chain order.
	Tgt []TgtEff
}

// TgtEff is one targeted SA's decision.TargetEffect facts plus what fits
// it to a kernel target slot.
type TgtEff struct {
	API uint8 // ApiOther, ApiDamage, ApiDamageAll, ApiCounter, ApiAttach, ...
	Dmg int16 // the literal damage amount, -1 unknown (DealDamage/DamageAll)
	// DmgK is the amount when the spell was kicked (Count$Kicked.A.B; Dmg
	// is then B); DmgX: the amount is the announced X.
	DmgK   int16
	DmgX   bool
	Remove uint8 // RmNone or the removal kind
	Boon   bool  // TargetEffect.Statics are all one-way boons (beneficialGrant)
	Mode   int8  // the charm mode whose chain holds it, -1 none
	// AttachPol is attachrules.go's attachPolarity of an Attach's AILogic$:
	// 1 a boon, -1 a curse, 0 unread.
	AttachPol int8
	// Fit: which controllers ValidTgts$ admits (sbtSlotFits): FitMine /
	// FitOpp bits for objects, FitPlayer when players are admitted.
	Fit uint8
}

// TargetEffect API codes.
const (
	ApiOther uint8 = iota
	ApiDamage
	ApiDamageAll
	ApiCounter
	ApiAttach
	ApiNone // no TargetEffect (nil)
)

// Removal kinds (RemovalEffect.Kind), ordered by removalRanker's bonus.
const (
	RmNone uint8 = iota
	RmExile
	RmCommand
	RmLibrary
	RmGraveyard
	RmDestroy
	RmSacrifice
	RmBounce
)

// Fit bits.
const (
	FitMine uint8 = 1 << iota
	FitOpp
	FitPlayerMine
	FitPlayerOpp
)

// NewFacts derives Facts from the kop table alone (no gorge strings): the
// printed facts the table carries; mana production from the mana
// abilities; no ability labels (every ability costs 20) and no target
// effects. HBotFacts (dirac/oracle) builds the exact ones.
func NewFacts(tbl *kop.Table) *Facts {
	f := &Facts{Tbl: tbl, Cards: make([]CardFacts, len(tbl.Cards)), contKw: true}
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		c := &f.Cards[i]
		c.CMC = int8(cd.Cost.MV())
		for j := 0; j < 5; j++ {
			c.Pips[j] = int8(cd.Cost.Pips[j])
		}
		c.HasX = cd.Cost.X != 0
		c.PerX = int8(cd.Cost.X)
		c.NoToughness = cd.Toughness <= 0
		c.Creature = cd.Types&kop.TCreature != 0
		c.Basic = cd.Types&kop.TBasic != 0
		c.Instant = cd.Types&kop.TInstant != 0
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			if a.Static.Kind == kop.StatCDAPT {
				c.CDA = true
			}
			if a.Static.Kind == kop.StatContinuous {
				c.ContKw |= a.Static.Kw
			}
		}
		c.Abilities = make([]AbFacts, len(cd.Abilities))
		converter := false
		c.FirstMana = -1
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			c.Abilities[j].Cost = 20
			c.Abilities[j].TapCost = a.Cost.Flags&kop.CostTap != 0
			switch a.Kind {
			case kop.AbMana:
				c.MakesMana = c.MakesMana || a.AFlags&kop.AfLatent == 0
				if c.FirstMana < 0 && a.AFlags&kop.AfLatent == 0 && a.ManaColors != 0 {
					c.FirstMana = int8(bits.TrailingZeros8(uint8(a.ManaColors)))
				}
				converter = converter || a.Cost.Mana != (kop.ManaCost{}) && a.Cost.Flags&kop.CostTap == 0
				if a.AFlags&kop.AfLatent == 0 && a.Cost.Mana == (kop.ManaCost{}) && a.Cost.Flags&^kop.CostTap == 0 {
					c.PlainMana |= uint8(a.ManaColors) & 0x1f
				}
				if a.ManaColors == kop.ColAny {
					c.ProdAny = true
					continue
				}
				for k := 0; k < 6; k++ {
					if a.ManaColors&(1<<k) != 0 && c.Produces[k] < int8(a.ManaN) {
						c.Produces[k] = int8(a.ManaN)
					}
				}
			case kop.AbSpell:
				for _, in := range a.Prog {
					if in.Op == kop.OpCounter {
						c.Counter = true
					}
				}
			}
		}
		c.Converter = converter
		c.EntryReturn = entryReturn(tbl, cd)
	}
	return f
}

// entryReturn is CardFacts.EntryReturn read off cd's lowered programs. Two
// shapes say it: a mandatory return of a land you control, which the entering
// land itself satisfies, and a sacrifice paid off by returning a permanent.
func entryReturn(tbl *kop.Table, cd *kop.CardDef) kop.FilterID {
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigETB || a.Trig.Flags&kop.TfSelf == 0 || len(a.Prog) == 0 {
			continue
		}
		switch in := &a.Prog[0]; in.Op {
		case kop.OpChooseMove:
			if in.Obj == kop.RefController && kop.Zone(in.C&7) == kop.ZoneHand && in.C>>3&kop.SrMandatory != 0 &&
				tbl.Filters[in.F] == (kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou}) {
				return in.F
			}
		case kop.OpUnlessPaid:
			if in.C&0xF == kop.PayReturn && len(a.Prog) > 1 && a.Prog[1].Op == kop.OpSacrifice {
				return in.F
			}
		}
	}
	return 0
}

// card is c's facts (zero facts for an unknown card).
func (f *Facts) card(c dirac.CardID) *CardFacts {
	if int(c) < len(f.Cards) {
		return &f.Cards[c]
	}
	return &zeroCard
}

var zeroCard CardFacts

// ab is card c's ability ab's facts (zero for an unknown one).
func (f *Facts) ab(c dirac.CardID, ab uint8) *AbFacts {
	cf := f.card(c)
	if int(ab) < len(cf.Abilities) {
		return &cf.Abilities[ab]
	}
	return &zeroAb
}

var zeroAb = AbFacts{Cost: 20}

// distinct is ManaProduction.DistinctColours: 5 for any colour, else how
// many of W U B R G the card produces (colourless not counted).
func (c *CardFacts) distinct() int {
	if c.ProdAny {
		return 5
	}
	n := 0
	for _, x := range c.Produces[:5] {
		if x > 0 {
			n++
		}
	}
	return n
}
