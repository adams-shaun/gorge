package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// playerCountBoundsCards are the ten supported cards this ticket moves off the
// two-seat set: each names a live count of opponents or players in a target
// bound or in an exile cost.
var playerCountBoundsCards = []string{
	"Age of Ultron",
	"Communal Brewing",
	"Hollow Marauder",
	"Rot Hulk",
	"Battle at the Helvault",
	"Cloud's Limit Break",
	"Lagrella, the Magpie",
	"Nils, Discipline Enforcer",
	"Thraben Charm",
	"Araumi of the Dead Tide",
}

// playerCountBoundsShape is what a carrier's live count lowers to: a target
// slot bounded at max with the different-controllers flag, a bounds-valued
// slot that evaluates the count when announced, or Araumi's live exile cost.
type playerCountBoundsShape struct {
	max     int  // the lowered target maximum (0: not a target bound)
	oppEach bool // "for each opponent": diff-controllers bounded at oppOneEachMax
	boundsV bool // a plain bounds-valued slot (TgfBoundsV)
	exile   bool // the live opponent-count exile cost
}

var playerCountBoundsShapes = map[string]playerCountBoundsShape{
	"Age of Ultron":             {max: oppOneEachMax, oppEach: true},
	"Communal Brewing":          {boundsV: true},
	"Hollow Marauder":           {boundsV: true},
	"Rot Hulk":                  {boundsV: true},
	"Battle at the Helvault":    {max: livingEachMax},
	"Cloud's Limit Break":       {max: livingEachMax},
	"Lagrella, the Magpie":      {max: livingEachMax},
	"Nils, Discipline Enforcer": {max: livingEachMax},
	"Thraben Charm":             {boundsV: true},
	"Araumi of the Dead Tide":   {exile: true},
}

// TestPlayerCountBounds: a target bound of the opponents' count
// (PlayerCountOpponents$Amount) or of the living players' count
// (PlayerCountPlayers$Amount), and Araumi's exile cost of the opponents'
// count, are live counts read at the game, not the two-seat constants 1 and
// 2. Each carrier lowers and builds with TwoSeat false and no reason: the
// plain bounds are TgfBoundsV slots the kernel evaluates, the for-each-player
// shapes and the different-controllers shapes are TgfDiffController slots the
// kernel's diffCtrlCap clamps to the live distinct controllers, and Araumi's
// cost carries kop.ExileGraveLiveCount.
func TestPlayerCountBounds(t *testing.T) {
	lw := lowerNamedPool(t, playerCountBoundsCards)
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	s, err := embeddedSupport()
	if err != nil {
		t.Fatal(err)
	}
	byLower := map[string]*lCard{}
	for _, c := range lw.cards {
		byLower[c.Name] = c
	}
	byDef := map[string]kop.CardDef{}
	for _, b := range bs {
		if len(b.refusals) != 0 {
			t.Errorf("%s: build refused: %v", b.def.Name, b.refusals)
		}
		byDef[b.def.Name] = b.def
	}
	for _, name := range playerCountBoundsCards {
		want := playerCountBoundsShapes[name]
		lc := byLower[name]
		if lc == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		if len(lc.Refusals) != 0 {
			t.Errorf("%s: refused: %v", name, lc.Refusals)
			continue
		}
		if st, _ := statusOf(lc, s); st != cardset.StatusSupported {
			t.Errorf("%s: status %v, want supported", name, st)
		}
		if lc.TwoSeat {
			t.Errorf("%s: still two-seat for %v", name, lc.TwoSeatWhy)
		}
		def, ok := byDef[name]
		if !ok {
			t.Errorf("%s: not built", name)
			continue
		}
		switch {
		case want.exile:
			checkLiveExileCost(t, name, lc, def)
		case want.boundsV:
			checkBoundsVSlot(t, name, lc, def)
		default:
			checkDiffCtrlSlot(t, name, lc, def, want.max)
		}
	}
}

// checkDiffCtrlSlot asserts the carrier has one different-controllers target
// slot bounded at max, in the lowering and in the built table.
func checkDiffCtrlSlot(t *testing.T, name string, lc *lCard, def kop.CardDef, max int) {
	t.Helper()
	foundLower := false
	for _, ab := range lc.Abilities {
		for i := range ab.Targets {
			tg := &ab.Targets[i]
			if !tg.DiffCtrl {
				continue
			}
			foundLower = true
			if !tg.Max.isConst() || tg.Max.N != max {
				t.Errorf("%s: diff-controllers target max %v, want %d", name, tg.Max, max)
			}
		}
	}
	if !foundLower {
		t.Errorf("%s: no different-controllers target lowered", name)
	}
	found := false
	for _, ab := range def.Abilities {
		for _, ts := range ab.Targets {
			if ts.TFlags&kop.TgfDiffController == 0 {
				continue
			}
			found = true
			if int(ts.Max) != max {
				t.Errorf("%s: built diff-controllers target max %d, want %d", name, ts.Max, max)
			}
		}
	}
	if !found {
		t.Errorf("%s: no different-controllers target built", name)
	}
}

// checkBoundsVSlot asserts the carrier's live-count maximum is a bounds-valued
// slot (TgfBoundsV) that reads the count, in the lowering and in the table.
func checkBoundsVSlot(t *testing.T, name string, lc *lCard, def kop.CardDef) {
	t.Helper()
	foundLower := false
	for _, ab := range lc.Abilities {
		for i := range ab.Targets {
			tg := &ab.Targets[i]
			if liveOppCount(tg.Max) || livingCount(tg.Max) {
				foundLower = true
			}
		}
	}
	if !foundLower {
		t.Errorf("%s: no live-count target maximum lowered", name)
	}
	found := false
	for _, ab := range def.Abilities {
		for _, ts := range ab.Targets {
			if ts.TFlags&kop.TgfBoundsV != 0 {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("%s: no bounds-valued target built", name)
	}
}

// checkLiveExileCost asserts Araumi's cost is CostExileGrave with the live
// opponent-count sentinel, not the fixed one-card exile of a two-seat card.
func checkLiveExileCost(t *testing.T, name string, lc *lCard, def kop.CardDef) {
	t.Helper()
	foundLower := false
	for _, ab := range lc.Abilities {
		if ab.Kind == akActivated && ab.Cost.ExileG != nil {
			foundLower = true
			if ab.Cost.ExileGN != kop.ExileGraveLiveCount {
				t.Errorf("%s: exile cost N %d, want the live opponent count %d", name, ab.Cost.ExileGN, kop.ExileGraveLiveCount)
			}
			if ab.Cost.ExileGX {
				t.Errorf("%s: exile cost is the announced X", name)
			}
		}
	}
	if !foundLower {
		t.Errorf("%s: no exile cost lowered", name)
	}
	found := false
	for _, ab := range def.Abilities {
		if ab.Cost.Flags&kop.CostExileGrave != 0 {
			found = true
			if ab.Cost.N != kop.ExileGraveLiveCount {
				t.Errorf("%s: built exile cost N %d, want %d", name, ab.Cost.N, kop.ExileGraveLiveCount)
			}
		}
	}
	if !found {
		t.Errorf("%s: no exile cost built", name)
	}
}
