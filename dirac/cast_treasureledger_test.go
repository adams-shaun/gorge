package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ktreScen is a scenario over the sacmana pool with an ExThisTurnCastSpec
// Treasure expression appended (the pool itself has no cast-ring reader), so
// prepareGen turns the ring on and marks the pool's Treasure token.
func ktreScen(t *testing.T) (*scen, func(string) CardID) {
	t.Helper()
	set := cardsettest.Set(t, "sacmana")
	src, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *src
	tbl.Exprs = append(append([]kop.Expr(nil), src.Exprs...), kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecTreasure})
	// The matchup's subtype vocabulary keeps only the subtypes a filter
	// names; this pool names no Treasure, so add the token's printed
	// subtype for the test (CastSpecTreasure reads it).
	tre := uint8(255)
	for i, n := range tbl.Subtypes {
		if n == "Treasure" {
			tre = uint8(i)
			break
		}
	}
	if tre == 255 {
		tbl.Subtypes = append(tbl.Subtypes, "Treasure")
		tre = uint8(len(tbl.Subtypes) - 1)
	}
	tok := set.PoolID(set.NumCards())
	tbl.Cards[tok].Subtypes = tbl.Cards[tok].Subtypes.With(int(tre))
	sc := newScenTable(t, &tbl, Config{})
	if !sc.k.gen.castRingOn {
		t.Fatal("the sacmana table does not turn the cast ring on")
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	return sc, id
}

// TestThisTurnCastTreasure: a spell whose planned payment taps a Treasure
// source marks that push castTreasurePush, and CastSpecTreasure reads
// the bit per push. A later cast paid only with lands does not inherit it.
func TestThisTurnCastTreasure(t *testing.T) {
	sc, id := ktreScen(t)
	bears := id("Grizzly Bears")
	forest := id("Forest")
	set := cardsettest.Set(t, "sacmana")
	treasure := CardID(set.PoolID(set.NumCards())) // c_a_treasure_sac, the first token
	x := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecTreasure}
	ctx := evalCtx{ctrl: 0}

	// One Forest + one Treasure pays {1}{G}: the planner's last resort.
	sc.perm(forest, 0)
	sc.perm(treasure, 0)
	b1 := sc.hand(bears, 0)
	sc.kcastBuild(b1, bears, 0)
	sc.s.manaStep(sc.k)
	sc.s.finishBuild(sc.k)
	if sc.s.castPushes(0)[0].prov&castTreasurePush == 0 {
		t.Fatal("a Treasure-paid cast is not marked")
	}
	if n := sc.s.thisTurnCastSpec(sc.k, x, ctx); n != 1 {
		t.Fatalf("CastSpecTreasure after a Treasure-paid cast: got %d, want 1", n)
	}

	// Two more Forests pay the second {1}{G} without a Treasure.
	sc.perm(forest, 0)
	sc.perm(forest, 0)
	b2 := sc.hand(bears, 0)
	sc.kcastBuild(b2, bears, 0)
	sc.s.manaStep(sc.k)
	sc.s.finishBuild(sc.k)
	if r := sc.s.castPushes(0); r[0].prov&castTreasurePush == 0 || r[1].prov&castTreasurePush != 0 {
		t.Fatalf("Treasure marks after a land-paid cast: %v, want the first cast's only", r)
	}
	if n := sc.s.thisTurnCastSpec(sc.k, x, ctx); n != 1 {
		t.Fatalf("CastSpecTreasure after a land-paid cast: got %d, want 1", n)
	}
}
