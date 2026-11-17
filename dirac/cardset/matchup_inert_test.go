package cardset

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// inertGuardPool is a three-card pool whose pair (Card B, Card C) leaves card 0
// out, so a compact CardID (0, 1) differs from the stable pool CardID (1, 2):
// MatchupInert must keep the game's compact ids, not Pool's.
func inertGuardPool() *PoolData {
	return &PoolData{Name: "inertguard", NumCards: 3, NumIDs: 3,
		Names: []string{"Card A", "Card B", "Card C"}, PoolID: []uint16{0, 1, 2},
		Status: make([]Status, 3), Reason: make([]string, 3),
		Defs: make([]kop.CardDef, 3), CardSubs: make([]PoolSubMask, 3), CardTokens: make([][]uint16, 3),
		CardRefSubs: make([][]uint8, 3), AbilityOrigin: make([][]uint8, 3), StaticSubs: make([][]AbSubs, 3),
		Filters: []kop.Filter{{}}, Exprs: []kop.Expr{}, Conds: []kop.Cond{}, Overrides: []kop.Override{}, Emblems: []kop.Emblem{}}
}

// A pair the interaction guard refuses builds through MatchupInert with the
// game's compact CardIDs, so a diagnostic run can replay it; a card the set
// does not play is still refused.
func TestMatchupInertSkipsGuards(t *testing.T) {
	withPairGuard(t, func(*kop.Table) *kop.PairRefusal {
		return &kop.PairRefusal{Reason: "testreason", Why: "because", Cards: []kop.CardID{0, 1}}
	})
	set := NewDataSet(inertGuardPool())
	a, b := []Entry{{Name: "Card B", Count: 1}}, []Entry{{Name: "Card C", Count: 1}}

	if _, _, _, err := set.Matchup(a, b); err == nil {
		t.Fatal("the guarded matchup built: the fixture does not reach the guard")
	} else {
		var r *InteractionRefusal
		if !errors.As(err, &r) || r.Reason != "testreason" {
			t.Fatalf("guarded matchup: %v, want the testreason InteractionRefusal", err)
		}
	}

	tb, da, db, err := set.MatchupInert(a, b)
	if err != nil {
		t.Fatalf("MatchupInert: %v, want the refused pair to build", err)
	}
	if tb == nil || len(da) != 1 || len(db) != 1 {
		t.Fatalf("MatchupInert returned table %v, ids %v %v", tb, da, db)
	}
	if da[0] != 0 || db[0] != 1 {
		t.Errorf("ids %v %v, want the compact game ids 0 and 1 (Pool's stable ids are 1 and 2)", da, db)
	}
	if len(tb.Cards) != 2 || tb.Cards[0].Name != "Card B" || tb.Cards[1].Name != "Card C" {
		t.Errorf("table cards %+v, want the pair's two compact defs", tb.Cards)
	}

	// It is not a by-name bypass: a card the set does not play still refuses.
	d := inertGuardPool()
	d.Status[0] = StatusNotLowered
	d.Reason[0] = "not lowered: fixture"
	_, _, _, err = NewDataSet(d).MatchupInert([]Entry{{Name: "Card A", Count: 1}}, b)
	var refused *RefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("MatchupInert with an unplayed card: %v, want a *RefusedError", err)
	}

	// MatchupNInert is the same arm for a pod.
	nt, ids, err := set.MatchupNInert([][]Entry{a, b})
	if err != nil {
		t.Fatalf("MatchupNInert: %v", err)
	}
	if len(nt.Cards) != 2 || len(ids) != 2 || ids[0][0] != 0 || ids[1][0] != 1 {
		t.Errorf("MatchupNInert: %d cards, ids %v", len(nt.Cards), ids)
	}
}
