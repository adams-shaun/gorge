package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
// This is a stop-gap for a model of alternative productions per unit across
// the planner, the window, the potential walk and tapFor; that work removes it.

// DebugLandGrantNoPairGuard is the mutation: the refusal is off, so the pair
// plays and stalls at the first meeting of the grant and the card (blgView).
var DebugLandGrantNoPairGuard bool

// landGrantScan is the colours the table's static grants add (the basic types
// of every Continuous static's and StatOverride static's AddSub, W U B R G as
// bits 0-4; a one-shot effect's override (Navigator's Compass, an animation) is
// no grant of the table and stays a runtime stall) and
// whether a grant reaches any card, not only lands: an override that adds the
// land type too (Ashaya, Soul of the Wild), an over-approximation of the
// granter's filter. g.blgSub is set (prepLandGrant).
func landGrantScan(t *kop.Table, g *genPrep) (granted kop.ColorMask, anyCard bool) {
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
				granted |= add(a.Static.AddSub)
			case kop.StatOverride:
				// a land-type setter (Blood Moon, CR 305.7) takes the land's own abilities
				// with it: one production, the intrinsic one (static_landtypesetter.go)
				if o := &t.Overrides[max(int(a.Static.Override), 1)-1]; a.Static.Override > 0 && !o.SetLandSub {
					c := add(o.AddSub)
					granted |= c
					anyCard = anyCard || c != 0 && o.AddTypes&kop.TLand != 0
				}
			}
		}
	}
	return granted, anyCard
}

// landGrantPairOn: t holds both halves of the land-grant refusal: a grant
// (landGrantScan) and a card that would stall under it (landGrantStalls).
// Called after prepLandGrant has set g.blgSub and g.blgIdx.
func landGrantPairOn(t *kop.Table, p *prep, g *genPrep) bool {
	if !g.blgOn {
		return false
	}
	granted, anyCard := landGrantScan(t, g)
	for i := range t.Cards {
		if landGrantStalls(t, p, g, i, granted, anyCard) {
			return true
		}
	}
	return false
}

// landGrantStalls: card i has no planner view (blgIdx < 0: its own mana
// ability is neither a plain {T} one nor a painland pair), is in reach of
// the grant, and would gain a granted colour its own ability does not make.
func landGrantStalls(t *kop.Table, p *prep, g *genPrep, i int, granted kop.ColorMask, anyCard bool) bool {
	if g.blgIdx[i] >= 0 || !anyCard && t.Cards[i].Types&kop.TLand == 0 {
		return false
	}
	return granted&^p.cards[i].manaCol&0x3f != 0
}

// landGrantPairText is StallLandGrant's refusal text and halves: side a the
// granters, side b the cards that stall under them.
func landGrantPairText(t *kop.Table, p *prep, g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	granted, anyCard := landGrantScan(t, g)
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
		func(_ *kop.CardDef, i int) bool { return landGrantStalls(t, p, g, i, granted, anyCard) }
}
