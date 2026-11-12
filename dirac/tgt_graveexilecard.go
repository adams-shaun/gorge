package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kop.TgtGraveExileCard, ChangeZone Origin$ Graveyard,Exile with its own
// ValidTgts$ (Sorceress's Schemes: "target instant or sorcery card from your
// graveyard or exiled card with flashback you own"). gorge
// changeZoneTargetZones names both zones in alternative order and each
// alternative matches only in its own zone. The one Filter serves both: each
// Or-chain clause carries its Zone (matchCardOneAt rejects a clause whose zone
// differs from the object's), so no zone code lives here.

// DebugGraveExileNoExile is TestGraveExileTargetedReturnMutation's switch:
// graveExileCardCands skips the exile walk.
var DebugGraveExileNoExile bool

// targetOKGraveExileCard: a graveyard or exile card matching t.Filter, with
// TgtCard's self rule (the card matcher reads neither self word).
//
//go:noinline
func (s *State) targetOKGraveExileCard(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, o Obj) bool {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneGraveyard && ob.zone != kop.ZoneExile {
		return false
	}
	if ob.zone == kop.ZoneGraveyard && s.graveCantTargetGen(k) {
		return false // Ground Seal's class (kw_canttgtgrave.go)
	}
	return s.matchCardTgt(k, t.Filter, ob.card, ob.owner(), p, o, src, self) &&
		(o != self || k.t.Filters[t.Filter].Flags&kop.FOther == 0)
}

// graveExileCardCands lists the choices: each graveyard in order (targetCands'
// TgtCard walk), then each exile list in order (gorge's zone-then-seat walk).
//
//go:noinline
func (s *State) graveExileCardCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]uint8, n int) int {
	for pp := 0; pp < MaxSeats; pp++ {
		pl := &s.pl[pp]
		for i := 0; i < int(pl.graveN); i++ {
			o := pl.grave[i]
			if uint8(o)+1 == s.g.tgtSelf {
				continue
			}
			if s.targetOK(k, t, p, src, self, uint8(o)) && n < MaxOptions-1 {
				out[n] = uint8(o)
				n++
			}
		}
	}
	if DebugGraveExileNoExile {
		return n
	}
	return s.exileCardCands(k, t, p, src, self, out, n) // filter_props.go
}
