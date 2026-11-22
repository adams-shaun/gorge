package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// StallLandGrant as a pair refusal (kop.PairGuard): a granted basic land type
// beside a mana source whose own ability is not a plain {T}: one colour
// (static_landtype.go blgPlain). gorge appends one intrinsic "{T}: Add [c]"
// beside the card's own mana abilities for each granted type its own do not
// make (CR 305.6), so City of Traitors under Cradle of Growth offers both {C}{C}
// and {G}; the kernel keeps one precomputed production per source and blgView
// stalls on it the moment the grant meets the card. The matchup is refused at
// its start instead (prep.landGrantPair, initStall). Yavimaya and Ashaya still
// play beside plain lands.
//
// A land whose own ability makes several mana (a bounce land, Temple of the
// False God) under one granted colour is modelled (static_landgrantalt.go) and
// no refusal; the shapes below stay refused until their alternatives are.

// DebugLandGrantNoPairGuard is the mutation: the refusal is off, so the pair
// plays and stalls at the first meeting of the grant and the card (blgView).
var DebugLandGrantNoPairGuard bool

// DebugLandGrantAnyReach is the mutation: a grant reaches every card, as the
// refusal did before it read the granter's filter (TestLandGrantReachMutation).
var DebugLandGrantAnyReach bool

// landGrantScan is the colours the table's static grants add (the basic types
// of every Continuous static's and StatOverride static's AddSub, W U B R G as
// bits 0-4; a one-shot effect's override (Navigator's Compass, an animation) is
// no grant of the table and stays a runtime stall) and
// the card types its grants reach: lands, and for an override that adds the land
// type too (Ashaya, Soul of the Wild) the types its filter names. g.blgSub is
// set (prepLandGrant).
func landGrantScan(t *kop.Table, g *genPrep) (granted kop.ColorMask, reach kop.TypeMask) {
	// reach is the card types the grants reach: kop.TLand always when a grant
	// is no card-wide override (it adds subtypes to lands), else the override
	// filters' types (filterTypeReach).
	add := func(m kop.SubMask) kop.ColorMask {
		var c kop.ColorMask
		for col, i := range g.blgSub {
			if i != subOutside && m.Has(int(i)) {
				c |= 1 << col
			}
		}
		return c
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			switch a.Static.Kind {
			case kop.StatContinuous:
				if c := add(a.Static.AddSub); c != 0 {
					granted |= c
					reach |= kop.TLand
				}
			case kop.StatOverride:
				// a land-type setter (Blood Moon, CR 305.7) takes the land's own abilities
				// with it: one production, the intrinsic one (static_landtypesetter.go)
				if o := &t.Overrides[max(int(a.Static.Override), 1)-1]; a.Static.Override > 0 && !o.SetLandSub {
					c := add(o.AddSub)
					granted |= c
					if c != 0 && o.AddTypes&kop.TLand != 0 {
						reach |= filterTypeReach(t, a.Static.Filter)
					} else if c != 0 {
						reach |= kop.TLand
					}
				}
			}
		}
	}
	return granted, reach
}

// filterTypeReach is the card types the filter chain at id can match: a card
// whose printed types miss every alternative's Types is out of its reach (Ashaya's
// "nontoken creatures you control" never reaches Sol Ring or a Signet, which are
// no creatures, CR 205.2). An alternative with no type constraint reaches every
// type. An animation that makes such a card a creature later is no static of
// the table: blgView then stalls it at runtime (StallLandGrant).
func filterTypeReach(t *kop.Table, id kop.FilterID) kop.TypeMask {
	var m kop.TypeMask
	for n := 0; id != 0 && n < len(t.Filters); n++ {
		f := &t.Filters[id]
		if f.Types == 0 {
			return ^kop.TypeMask(0)
		}
		m |= f.Types
		id = f.Or
	}
	return m
}

// landGrantPairOn: t holds both halves of the land-grant refusal: a grant
// (landGrantScan) and a card that would stall under it (landGrantStalls).
// Called after prepLandGrant has set g.blgSub and g.blgIdx.
func landGrantPairOn(t *kop.Table, p *prep, g *genPrep) bool {
	if !g.blgOn {
		return false
	}
	granted, reach := landGrantScan(t, g)
	for i := range t.Cards {
		if landGrantStalls(t, p, g, i, granted, reach) {
			return true
		}
	}
	return false
}

// landGrantStalls: card i has no planner view (blgIdx < 0: its own mana
// ability is neither a plain {T} one nor a painland pair), is in reach of the grant, and would gain a
// granted colour its own ability does not make; or it grants a mana ability
// (grantsManaTo) to a card the land-type grant reaches.
func landGrantStalls(t *kop.Table, p *prep, g *genPrep, i int, granted kop.ColorMask, reach kop.TypeMask) bool {
	if DebugLandGrantOwnOff && grantsManaTo(t, p, g, i, reach) {
		return true
	}
	if t.Cards[i].Types&reach == 0 && !DebugLandGrantAnyReach {
		return false
	}
	return g.blgIdx[i] < 0 && granted&^ownMadeCols(&p.cards[i])&0x3f != 0 && !altCovers(g, p, i, granted) && !gateCovers(g, i)
}

// grantsManaTo: card i carries a latent mana ability (Citanul Hierophants'
// "{T}: Add {G}" to every creature) and the land-type grant reaches a card with
// no mana ability of its own. Such a card took the granted view, which holds no
// intrinsic production (manaPCGen, blgStall), and the planner read it as a
// classless granted unit; gorge's census counts the intrinsic ability as the
// card's own (blgOwnUsable makes it so here), so the refusal holds only with
// DebugLandGrantOwnOff.
func grantsManaTo(t *kop.Table, p *prep, g *genPrep, i int, reach kop.TypeMask) bool {
	granter := false
	for _, r := range g.grantOf {
		granter = granter || int(r.card) == i
	}
	if !granter {
		return false
	}
	for j := range t.Cards {
		if t.Cards[j].Types&reach != 0 && p.cards[j].manaAb < 0 && p.cards[j].manaCol == 0 {
			return true
		}
	}
	return false
}

// landGrantPairText is StallLandGrant's refusal text and halves: side a the
// granters, side b the cards that stall under them.
func landGrantPairText(t *kop.Table, p *prep, g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	granted, reach := landGrantScan(t, g)
	grants := func(m kop.SubMask) bool {
		for _, i := range g.blgSub {
			if i != subOutside && m.Has(int(i)) {
				return true
			}
		}
		return false
	}
	return "a granted basic land type beside a mana source whose own ability is not a plain {T}: gorge offers both productions",
		abilityOn(func(x *kop.Ability) bool {
			if x.Kind != kop.AbStatic {
				return false
			}
			switch x.Static.Kind {
			case kop.StatContinuous:
				return grants(x.Static.AddSub)
			case kop.StatOverride:
				return x.Static.Override > 0 && grants(t.Overrides[x.Static.Override-1].AddSub)
			}
			return false
		}),
		func(_ *kop.CardDef, i int) bool { return landGrantStalls(t, p, g, i, granted, reach) }
}

// ownMadeCols is the colours a view's own mana ability makes as far as gorge's
// intrinsic dedup sees them: it appends a granted type's ability unless some
// ability has Produced$ exactly that colour (pay.ManaAbilitiesProduce keys on
// the raw text). An all-at-once ability (a bounce land's "G U", Arixmethes) is
// no single colour's, so it makes none: Simic Growth Chamber under a granted
// Island still has the intrinsic {U} beside {G}{U}.
func ownMadeCols(pc *pcard) kop.ColorMask {
	for _, n := range pc.manaXv {
		if n > 0 {
			return 0
		}
	}
	if !pc.manaTap && pc.manaAb >= 0 && bits.OnesCount8(uint8(pc.manaCol)&0x3f) > 1 {
		return 0 // a paid ability making several colours (a filter's "Add {W}{U}") is no colour's own either (static_landtype.go blgFilterFirst)
	}
	return pc.manaCol
}
