package cardset

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// guardPool is a small pool: two cards and a token only the second makes.
func guardPool() *PoolData {
	return &PoolData{Name: "guard", NumCards: 2, NumTokens: 1, NumIDs: 3,
		Names: []string{"Card A", "Card B", "Token T"}, PoolID: []uint16{0, 1, 2}, Status: make([]Status, 3), Reason: make([]string, 3),
		Defs: make([]kop.CardDef, 3), CardSubs: make([]kop.SubMask, 3), CardTokens: [][]uint16{nil, {2}, nil},
		CardRefSubs: make([][]uint8, 3), AbilityOrigin: make([][]uint8, 3), StaticSubs: make([][]AbSubs, 3),
		Filters: []kop.Filter{{}}, Exprs: []kop.Expr{}, Conds: []kop.Cond{}, Overrides: []kop.Override{}, Emblems: []kop.Emblem{}}
}

// Not parallel: it sets kop.PairGuard, and a serial test has the binary to itself.
func withPairGuard(t *testing.T, g func(*kop.Table) *kop.PairRefusal) {
	t.Helper()
	old := kop.PairGuard
	t.Cleanup(func() { kop.PairGuard = old })
	kop.PairGuard = g
}

// A guard's refusal comes back typed, with the carriers by name, a token by the
// deck card that makes it, and nothing built.
func TestMatchupRefusesWhatTheGuardRefuses(t *testing.T) {
	withPairGuard(t, func(tb *kop.Table) *kop.PairRefusal {
		return &kop.PairRefusal{Reason: "testreason", Why: "because", Cards: []kop.CardID{0, 2}}
	})
	tb, da, db, err := NewDataSet(guardPool()).Matchup([]Entry{{Name: "Card A", Count: 1}}, []Entry{{Name: "Card B", Count: 1}})
	var r *InteractionRefusal
	if !errors.As(err, &r) {
		t.Fatalf("err %v, want an InteractionRefusal", err)
	}
	if tb != nil || da != nil || db != nil {
		t.Error("a refused matchup returns nothing to play")
	}
	if r.Reason != "testreason" || len(r.Cards) != 2 || r.Cards[0] != "Card A" || r.Cards[1] != "Card B" {
		t.Errorf("refusal %+v", r)
	}
	if want := `interaction testreason: Card A, Card B: because`; err.Error() != want {
		t.Errorf("message %q, want %q", err, want)
	}
}

// No guard, a guard with nothing to say, and a pool table all build.
func TestInteractionRefusalOnlyWhenTheGuardSays(t *testing.T) {
	es := []Entry{{Name: "Card A", Count: 1}}
	withPairGuard(t, nil)
	if _, _, _, err := NewDataSet(guardPool()).Matchup(es, es); err != nil {
		t.Errorf("no guard: %v", err)
	}
	withPairGuard(t, func(*kop.Table) *kop.PairRefusal { return nil })
	if _, _, _, err := NewDataSet(guardPool()).Matchup(es, es); err != nil {
		t.Errorf("nothing to say: %v", err)
	}
	withPairGuard(t, func(*kop.Table) *kop.PairRefusal {
		t.Error("the guard read a pool table")
		return nil
	})
	if _, err := NewDataSet(guardPool()).Pool(); err != nil {
		t.Errorf("Pool: %v", err)
	}
}
