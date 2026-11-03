package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// graveAggs are the Count$ValidGraveyard $<prop> folds the kernel's
// ExGraveAgg answers (gorge count_zone_fold.go zoneCountFold.visit): the
// mana-value ones, read through state.ManaValue (printed; a split card in
// the graveyard is its halves' sum), and the card-type sets (CardTypes,
// CardTypesPermanent: the face's printed types). The
// power/toughness folds read a graveyard card's characteristics through the
// layer walk and stay refused, as do the name and colour sets.
var graveAggs = map[string]int{
	"GreatestCardManaCost":  kop.AggGreatestMV,
	"CardManaCost":          kop.AggSumMV,
	"DifferentCardManaCost": kop.AggDistinctMV,
	"CardTypes":             kop.AggCardTypes,
	"CardTypesPermanent":    kop.AggPermTypes,
	"GreatestCardPower":     kop.AggGreatestPower, // the card's printed power; a `*` power rides the kernel's CDA read
}

// validAggs are the Count$Valid $<prop> folds the kernel's ExValidAgg
// answers over the battlefield (gorge count_body_zone.go zoneCountFold).
var validAggs = map[string]int{
	"GreatestCardPower":     kop.AggGreatestPower,
	"GreatestCardToughness": kop.AggGreatestToughness,
	"GreatestCardManaCost":  kop.AggGreatestMV,
	"CardToughness":         kop.AggSumToughness,
	"CardManaCost":          kop.AggSumMV,
	"DifferentCardPower":    kop.AggDistinctPower,
	"Colors":                kop.AggColors,
	"DifferentCardNames":    kop.AggDistinctNames,
	"CardTypes":             kop.AggCardTypes,
	"CreatureType":          kop.AggCreatureTypes,
}

// ptAggIn reports whether v reads a derived P/T aggregate (a CDA reading
// one would recurse through the kernel's layer walk; refused).
func ptAggIn(v lValue) bool {
	if v.Kind == vValidAgg {
		switch v.N {
		case kop.AggGreatestPower, kop.AggGreatestToughness, kop.AggSumToughness, kop.AggDistinctPower:
			return true
		}
	}
	for _, s := range v.Sub {
		if ptAggIn(s) {
			return true
		}
	}
	return false
}
