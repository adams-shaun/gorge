package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGraveyardCastNeedAnnouncesTheCastsCost pins the plain graveyard cast's
// need (graveyardCastNeedCost through playNeed): the hint names the pips of
// the cost the cast prices -- a mayhem or harmonize cast's printed
// alternative cost, not the card's printed mana cost -- while playBaseCost
// still declines the cast, so the pricing reads never see a graveyard cast's
// cost (mana_filterrepeat.go filtUsefulCost, mana_grantsubsume.go
// playCostCounts). The mayhem and harmonize costs are recoloured, so the
// announced colour distinguishes the cast's cost from the printed one.
func TestGraveyardCastNeedAnnouncesTheCastsCost(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Rocket-Powered Goblin Glider", Count: 2},
		{Name: "Unending Whisper", Count: 2},
		{Name: "Tormenting Voice", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Island", Count: 4},
	}
	set := cardsettest.Set(t, "delvegrant")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// recolour the mayhem and harmonize costs: {3} and {2}{U} printed stay,
	// the casts' own costs name W and B (kop/table.go prices both at AltCost)
	glider := &tbl.Cards[id["Rocket-Powered Goblin Glider"]]
	if !glider.Mayhem {
		t.Fatalf("precondition: Rocket-Powered Goblin Glider is no mayhem card")
	}
	glider.AltCost = kop.ManaCost{Pips: func() [kop.NumManaKinds]uint8 {
		var p [kop.NumManaKinds]uint8
		p[kop.W] = 1
		return p
	}()}
	whisper := &tbl.Cards[id["Unending Whisper"]]
	if !whisper.Harmonize {
		t.Fatalf("precondition: Unending Whisper is no harmonize card")
	}
	whisper.AltCost = kop.ManaCost{Pips: func() [kop.NumManaKinds]uint8 {
		var p [kop.NumManaKinds]uint8
		p[kop.B] = 1
		return p
	}()}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})

	t.Run("mayhem", func(t *testing.T) {
		rgO := sc.grave(id["Rocket-Powered Goblin Glider"], 0, 1)[0]
		a := MakeAction(AkCast, TCode(rgO), 0, 2)
		if _, _, ok := sc.s.playBaseCost(sc.k, a); ok {
			t.Fatal("precondition: playBaseCost prices the mayhem cast, so the test does not reach the hint")
		}
		need := sc.s.playNeedHint(sc.k, a)
		if !need[kop.W] || need[kop.B] || need[kop.C] {
			t.Fatalf("playNeedHint of the mayhem cast %v, want the mayhem cost's {W} pip alone", need)
		}
	})
	t.Run("harmonize", func(t *testing.T) {
		whO := sc.grave(id["Unending Whisper"], 0, 1)[0]
		a := MakeAction(AkCast, TCode(whO), 0, 2)
		if _, _, ok := sc.s.playBaseCost(sc.k, a); ok {
			t.Fatal("precondition: playBaseCost prices the harmonize cast, so the test does not reach the hint")
		}
		need := sc.s.playNeedHint(sc.k, a)
		if !need[kop.B] || need[kop.U] || need[kop.C] {
			t.Fatalf("playNeedHint of the harmonize cast %v, want the harmonize cost's {B} pip alone", need)
		}
	})
}
