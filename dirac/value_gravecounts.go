package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// graveTypes counts the card types (land through kindred, CR 300.1) among
// the printed types of the cards in player p's graveyard (gorge
// graveyardCardTypeCount; delirium is four or more).
//
//go:noinline
func (s *State) graveTypes(k *prep, p uint8, uncounted Obj) int {
	const cardTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
		kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred
	var m kop.TypeMask
	pl := &s.pl[p]
	for i := 0; i < int(pl.graveN); i++ {
		if o := pl.grave[i]; uncounted != o+1 {
			m |= k.t.Cards[s.objs[o].card].Types & cardTypes
		}
	}
	return bits.OnesCount32(uint32(m))
}

// graveCards counts the cards in player p's graveyard. A token or copy that
// has reached the graveyard waits there for the next state-based check
// (CR 704.5d) and is no card (CR 111.7), so it does not count; gorge's
// state.Game.CardCount makes the same exclusion.
func (s *State) graveCards(p uint8) int {
	n := 0
	pl := &s.pl[p]
	for i := 0; i < int(pl.graveN); i++ {
		if s.objs[pl.grave[i]].of&(ofToken|ofCopy) == 0 {
			n++
		}
	}
	return n
}

// Count$ValidGraveyard <spec>$CardTypes and
// $CardTypesPermanent (Tarmogoyf, Broodspinner, Matzalantli). Cold code,
// out of line.

// permTypeBits are CR 110.4's permanent types, gorge's permanentTypeWords
// (count_zone_fold.go's CardTypesPermanent vocabulary).
const permTypeBits = kop.TArtifact | kop.TBattle | kop.TCreature | kop.TEnchantment | kop.TLand | kop.TPlaneswalker

// The graveyard fold counts cardTypeBits (value_validagg.go): count_zone_fold.go's
// zoneFoldPropCardTypes vocabulary, gorge's cardTypeWords map
// (creature_types.go:123), which holds "Kindred" (CR
// 205.2a, 308.3: Kindred is a card type). Before that fix the map lacked it
// and the kernel masked TKindred out to copy the gap; the fold is now the
// same mask as the battlefield one.

// DebugGraveTypesAll is TestGravecountsMutation's switch: the
// CardTypesPermanent fold counts every card type (instants and sorceries
// too). Never set outside that test.
var DebugGraveTypesAll bool

// StallDynMana's name, registered cold-side per the
// convention: gen.go's manaPCGen stalls a dynamic mana amount above
// maxDynMana instead of truncating it (The Core reached 21).
func init() { stallNames[StallDynMana] = "dynmana" }

// graveTypeAgg is ExGraveAgg's AggCardTypes / AggPermTypes arm: gorge
// zoneCountFold.visit's zoneFoldPropCardTypes over the graveyard cards
// matching F, the number of distinct card types (CR 205.2a) among their
// faces' printed types (gorge o.Face().Types; graveTypes' read, the
// card's Types).
//
//go:noinline
func (s *State) graveTypeAgg(k *prep, x *kop.Expr, ctx evalCtx) int {
	mask := kop.TypeMask(cardTypeBits)
	if x.N == kop.AggPermTypes && !DebugGraveTypesAll {
		mask = permTypeBits
	}
	var types kop.TypeMask
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[pp]
		for j := 0; j < int(pl.graveN); j++ {
			o := pl.grave[j]
			if s.objs[o].of&ofToken != 0 {
				continue // a token that died waits there until the next check ceases it; it is no card (CR 111.7, 704.5d)
			}
			c := s.objs[o].card
			if s.matchCardGenAt(k, x.F, c, pp, ctx.ctrl, o, &ctx) {
				types |= k.t.Cards[c].Types
			}
		}
	}
	return bits.OnesCount16(uint16(types & mask))
}
