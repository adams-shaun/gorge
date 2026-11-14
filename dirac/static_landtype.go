package dirac

import (
	"math/bits"
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// BlgStats counts granted basic-land-type mana views; NonLand counts the
// ones for cards not printed as lands (the Ashaya class).
var BlgStats struct{ NonLand atomic.Int64 }

// A granted basic land type (CR 305.6): a Continuous static's
// AddType$ AllBasicLandType (Leyline of the Guildpact, Prismatic Omen, Dryad
// of the Ilysian Grove, Nylea's Presence) gives a land the five basic land
// types, and a land with a basic land type has its intrinsic "{T}: Add [c]"
// ability. The subtypes themselves are the static's AddSub (hasSubGen); this
// file is the mana half. gorge appends the intrinsic ability per derived
// basic type its listed abilities do not already produce
// (rules/mana_activation.go appendAvailableManaAbilitiesGate, the same list
// its planner census reads: potential_plan.go paymentPlanCensusOf), printed
// abilities first, then Plains Island Swamp Mountain Forest.
//
// The kernel's unit is the source: manaPCGen returns a precomputed view of
// the card's own facts with the granted colours ORed into manaCol, placed
// with the fixed-production units like a printed dual (manaMulti), and
// blgFirst naming its first ability's colour for the payment window (the
// printed ability's, else the lowest granted one). One source is one tap,
// so the union is only valid when every own mana ability is a plain {T}
// ability of one colour making one mana (prepMultiMana's shape), or there
// is none: any other affected source stalls StallLandGrant.

// StallLandGrant: a card with a granted basic land type whose own mana
// ability is not the plain {T}-for-one-mana shape (a {T}: Add {C}{C} land,
// a painland, a chosen, restricted, dynamic or granted ability), or that
// lost its abilities: one source would have to offer two different
// productions, which the one-ability pcard cannot hold.
const StallLandGrant = 120

func init() { stallNames[StallLandGrant] = "landgrant" }

// blgNames are the basic land types in gorge's appendLandTypes order, the
// colour order W U B R G of their intrinsic abilities.
var blgNames = [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}

// prepLandGrant finds the five basic land subtypes in the matchup table and
// sets g.blgOn when some Continuous static or override adds one; it then
// builds the 32 planner views of every plain-shaped card, lands and the
// creatures an override makes Forest lands (Ashaya, Soul of the Wild, Life
// and Limb) alike: gorge appends the intrinsic "{T}: Add [c]" for every
// derived basic type word on the battlefield (mana_activation.go's CR 305.6
// block), and a non-token creature under Ashaya's grant is one.
func prepLandGrant(t *kop.Table, p *prep, g *genPrep) {
	var basic kop.SubMask
	for c := range g.blgSub {
		g.blgSub[c] = subOutside
		for i, n := range t.Subtypes {
			if n == blgNames[c] {
				g.blgSub[c] = uint8(i)
				basic = basic.With(i)
			}
		}
	}
	meets := func(m kop.SubMask) bool { return m[0]&basic[0]|m[1]&basic[1] != 0 }
	on := false
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			on = on || a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && meets(a.Static.AddSub)
		}
	}
	for i := range t.Overrides {
		on = on || meets(t.Overrides[i].AddSub)
	}
	if !on {
		return
	}
	g.blgOn = true
	g.blgIdx = make([]int16, len(t.Cards))
	for i := range t.Cards {
		g.blgIdx[i] = -1
		base := p.cards[i]
		if !blgPlain(&t.Cards[i], &base) {
			continue
		}
		g.blgIdx[i] = int16(len(g.blgPC) / 32)
		for m := 0; m < 32; m++ {
			v := base
			first := bits.TrailingZeros8(uint8(m))
			if base.manaAb >= 0 {
				first = bits.TrailingZeros8(uint8(base.manaCol & 0x3f))
				if ab := &t.Cards[i].Abilities[base.manaAb]; ab.AFlags&kop.AfManaChoice != 0 {
					var ord [5]uint8
					if manaColourOrder(ab, &ord) > 0 {
						first = int(ord[0]) // a Combo answers its printed token order (Combo G W: {G})
					}
				}
				v.manaCol |= kop.ColorMask(m)
			} else {
				v.manaAb = 0 // no own ability: the intrinsic one (manaAbView guards the reads)
				v.manaCol = kop.ColorMask(m)
			}
			v.manaN, v.manaTap = 1, true
			v.manaMulti = bits.OnesCount8(uint8(v.manaCol&0x3f)) > 1
			v.blgFirst = uint8(first + 1)
			g.blgPC = append(g.blgPC, v)
		}
	}
}

// blgPrintedMask is the colours of the own mana abilities of the land at sl that
// gorge calls printed, 0 for a basic: gorge's PaymentAbility calls an ability
// derived from a basic land type intrinsic, as it does a granted one, and the
// script of a dual (Commercial District: Land Mountain Forest) prints none, so
// its {R} and {G} are intrinsic and rank beside the granted colours, not after.
// Only an ability a basic subtype of the card does not account for is printed.
func (s *State) blgPrintedMask(k *prep, sl Slot) uint8 {
	c := s.cardAt(sl)
	own := &k.cards[c]
	if own.types&kop.TBasic != 0 || own.manaAb < 0 || s.landSetLost(k, sl) {
		return 0 // a land-type setter took the printed abilities (static_landtypesetter.go)
	}
	m := uint8(own.manaCol & 0x3f)
	for col, i := range k.gen.blgSub {
		if i != subOutside && k.t.Cards[c].Subtypes.Has(int(i)) {
			m &^= 1 << col
		}
	}
	return m
}

// blgPlain reports whether card c's own mana abilities (pc its facts) are
// all plain {T} abilities making one mana of one colour, or it has none. A
// later ability the planner census misses (a {1},{T} filter, Rumble Arena) or
// counts per payment (a restricted one, Jasmine Dragon Tea Shop) is not part of
// the view: the gap and restricted-source models read it from the card.
func blgPlain(c *kop.CardDef, pc *pcard) bool {
	if pc.manaChosen || pc.manaDyn || pc.painCol != 0 || pc.ctrMana {
		return false
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if j != int(pc.manaAb) && (censusGapMana(a) || restrSrcAb(a)) {
			continue // a costed or restricted ability beside the first: the census gap / restricted-source machinery's, not a view colour
		}
		if a.AFlags&^kop.AfManaChoice != 0 || a.ManaRestrict != 0 || a.ManaN > 1 || a.Cost.Flags != kop.CostTap || a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 ||
			a.ManaColors&0x3f == 0 || a.AFlags&kop.AfManaChoice == 0 && bits.OnesCount8(uint8(a.ManaColors&0x3f)) != 1 || len(a.Prog) != 0 || a.Trig.Cond != 0 {
			return false
		}
	}
	return true
}

// blgMask is the colours of the intrinsic abilities slot sl's granted
// basic land types give it: a bit per basic type it has, when it is a land.
//
//go:noinline
func (s *State) blgMask(k *prep, sl Slot) kop.ColorMask {
	if s.typesAt(k, sl)&kop.TLand == 0 || s.genLost(k, sl) && !s.landSetLost(k, sl) {
		return 0 // a layer-6 loss takes the intrinsic abilities too, whatever its timestamp (gorge: lossStamp != 0, mana_activation.go); a land-type setter's does not
	}
	var m kop.ColorMask
	for c, i := range k.gen.blgSub {
		if i != subOutside && s.hasSubGen(k, sl, i) {
			m |= 1 << c
		}
	}
	return m
}

// blgView is manaPCGen's view of slot sl (card c, own facts pc) on a table
// with a basic land type grant: pc unless a granted type adds a colour pc
// does not make.
//
//go:noinline
func (s *State) blgView(k *prep, sl Slot, c CardID, pc *pcard) *pcard {
	m := s.blgMask(k, sl)
	if pc.manaAb >= 0 {
		m &^= pc.manaCol // gorge ManaAbilitiesProduce: a printed colour is not appended again
	}
	if m == 0 {
		return pc
	}
	g := k.gen
	if g.blgIdx[c] < 0 {
		s.capFull(StallLandGrant)
		return pc
	}
	if k.t.Cards[c].Types&kop.TLand == 0 {
		BlgStats.NonLand.Add(1) // an Ashaya-class grant on a non-land card
	}
	return &g.blgPC[int(g.blgIdx[c])*32+int(m)]
}

// blgStall stalls when slot sl (card c) gains an intrinsic ability its own
// abilities do not make on a manaPCGen path the union view does not cover
// (a granted, gated or dynamic ability).
//
//go:noinline
func (s *State) blgStall(k *prep, sl Slot, c CardID) {
	if s.blgMask(k, sl)&^k.cards[c].manaCol != 0 {
		s.capFull(StallLandGrant)
	}
}

// hybStrand is tapForHyb's stranded window. On a table with a basic land type
// grant, a granted source answers its printed ability first (gorge's announced
// window lists its abilities in order and the oracle's payWindowAnswer takes
// the first), so the window spends a Forest on {G} though the cost owes {B},
// and runs out of sources. gorge then cancels the cast and keeps the taps and
// the floated mana (CR 733.1), as for a painland (loweredWindowOK). It reports
// false after reversing that way; the real walk can also complete the cost the
// dry census could not price (a filter's fee paid from the pool it already
// floated, Temur Devotee), and it reports true then, keeping the floated mana.
//
//go:noinline
func (s *State) hybStrand(k *prep, p uint8, c *kop.ManaCost, anyColor bool) bool {
	ok, held := s.windowTapRider(k, p, c, true, anyColor)
	for col := range held {
		s.pl[p].pool[col] += held[col]
	}
	if ok {
		// the real walk completed the cost the dry census could not price: a
		// filter's fee paid from the pool it already floated (Temur Devotee
		// beside a {2/G}{2/U}{2/R} cast)
		return true
	}
	s.reverseCast(k)
	return false
}

// manaAbView is the card ability a view's manaAb names, nil when it names
// none. A no-own-ability granted-type view sets manaAb to 0 for the intrinsic
// ability (prepLandGrant above), so a card with no abilities has no index 0
// for a reader to dereference; the readers guard through this helper.
func manaAbView(cd *kop.CardDef, pc *pcard) *kop.Ability {
	if pc.manaAb < 0 || int(pc.manaAb) >= len(cd.Abilities) {
		return nil
	}
	return &cd.Abilities[pc.manaAb]
}

// potentialFirstMask is the colours a multi-colour source adds for its first
// mana ability in the potential walk (availPotentialFor), which then admits the
// card's other table abilities one by one. A granted basic land type's intrinsic
// ability is in no table (Leyline of the Guildpact over a Swamp or Stomping
// Ground: gorge counts the printed abilities and one per granted type, CR 305.6),
// so the colours the view holds beyond the card's own mana abilities belong here.
func (s *State) potentialFirstMask(k *prep, sl Slot, pc *pcard) uint8 {
	cd := &k.t.Cards[s.cardAt(sl)]
	var mask uint8
	if ab := manaAbView(cd, pc); ab != nil {
		mask = uint8(ab.ManaColors)
	}
	if k.gen == nil || !k.gen.blgOn {
		return mask
	}
	var own uint8
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && !(pc.manaAb >= 0 && manaAbView(cd, pc) == nil) {
			own |= uint8(a.ManaColors & 0x3f) // an intrinsic-only view has none (static_landtypesetter.go)
		}
	}
	return mask | uint8(pc.manaCol&0x3f)&^own
}
