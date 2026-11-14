package oracle

import (
	"reflect"
	"slices"
	"testing"
)

// searchSeatGame plays pair p of the FDN pool in lockstep with the seats of
// cfg answered by dirac/search and seat 1 by gorge's heuristic bot.
func searchSeatGame(t *testing.T, c *Corpus, p Pairing, cfg SearchSeatConfig) (Result, Policy) {
	t.Helper()
	set := rawSet("fdn")
	m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
	if err != nil {
		t.Fatalf("pair %d: %v", p.Index, err)
	}
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true, PassElision: true}, MaxDecisions: 20000, MaxTurns: 100}
	mk, pol := WithSearchSeats(SetSide(set, tbl, rc.Config), HeuristicPolicy(p.Seed), cfg, c.Reg, set)
	k, err := mk(m, p.Seed)
	if err != nil {
		t.Fatalf("pair %d: %v", p.Index, err)
	}
	defer CloseSide(k)
	return Lockstep(m, p.Seed, k, pol, rc), pol
}

// TestSearchSeatPlaysFDNPair: two FDN-pool pairs with seat 0 answered by
// dirac/search (16 simulations) and seat 1 by the heuristic bot end with no
// divergence, the search seat searched, and a second run plays the same
// actions.
func TestSearchSeatPlaysFDNPair(t *testing.T) {
	c, pool := testCorpus(t)
	cfg := SearchSeatConfig{Seats: 1, Sims: 16}
	for _, p := range PairingsFrom(pool, 0, 2, 78) {
		res, pol := searchSeatGame(t, c, p, cfg)
		if res.Div != nil {
			t.Fatalf("pair %d diverged: %s", p.Index, res.Div)
		}
		by := SearchedBy(pol)
		if by[0] == 0 {
			t.Fatalf("pair %d: seat 0 searched no decision (%d decisions, stall %q)", p.Index, res.Decisions, res.Stall)
		}
		if by[1] != 0 {
			t.Errorf("pair %d: seat 1 is not a search seat but searched %d", p.Index, by[1])
		}
		again, pol2 := searchSeatGame(t, c, p, cfg)
		if !slices.Equal(res.Actions, again.Actions) || SearchedBy(pol2) != by {
			t.Errorf("pair %d: a second run played different actions (%d vs %d), searched %v vs %v", p.Index, len(res.Actions), len(again.Actions), by, SearchedBy(pol2))
		}
		t.Logf("pair %d: %d decisions, searched %v, fallbacks %d", p.Index, res.Decisions, by, PolicyFallbacks(pol))
	}
}

// TestSearchSeatOffIsUnchanged: no search seat returns the factory and the
// policy it was given, and the game is the plain Lockstep's.
func TestSearchSeatOffIsUnchanged(t *testing.T) {
	c, pool := testCorpus(t)
	set := rawSet("fdn")
	p := PairingsFrom(pool, 0, 1, 78)[0]
	m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true, PassElision: true}, MaxDecisions: 20000, MaxTurns: 100}
	play := func(wrap bool) Result {
		mk, pol := SetSide(set, tbl, rc.Config), HeuristicPolicy(p.Seed)
		if wrap {
			mk2, pol2 := WithSearchSeats(mk, pol, SearchSeatConfig{Sims: 16}, c.Reg, set)
			if pol2 != pol {
				t.Fatal("no search seat returned a different policy")
			}
			if r1, r2 := reflectPtr(mk), reflectPtr(mk2); r1 != r2 {
				t.Fatal("no search seat returned a different factory")
			}
			mk = mk2
		}
		k, err := mk(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		defer CloseSide(k)
		return Lockstep(m, p.Seed, k, pol, rc)
	}
	plain, wrapped := play(false), play(true)
	if plain.Div != nil || plain.Decisions == 0 {
		t.Fatalf("the plain game: %d decisions, div %v", plain.Decisions, plain.Div)
	}
	if !slices.Equal(plain.Actions, wrapped.Actions) || plain.Winner != wrapped.Winner || plain.Decisions != wrapped.Decisions {
		t.Fatalf("a game with no search seat differs: %d vs %d decisions", plain.Decisions, wrapped.Decisions)
	}
}

// reflectPtr is a func value's code pointer, to tell two factories apart.
func reflectPtr(f SideFactory) uintptr { return reflect.ValueOf(f).Pointer() }
