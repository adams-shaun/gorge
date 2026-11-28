package oracle

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAtomDiffSetBases: every registered set atom (one per kernel base kind and
// combinator) agrees with gorge's Defined$ at every compared decision of two
// FDN two-seat games and a four-seat commander pod, and each reads a non-empty set (a stack-top context included, so the target and trigger bases
// read real referents). A player base is never empty, so only a non-empty
// reading is required of every atom.
func TestAtomDiffSetBases(t *testing.T) {
	atoms := RegisteredSetAtoms()
	if len(atoms) < 13 {
		t.Fatalf("precondition: %d set atoms registered, want the 8 bases and 5 combinators", len(atoms))
	}
	st := NewAtomStats()
	var fails []string
	games := 0
	play := func(name string, m *Matchup, seed uint64, k Side, cfg Config) {
		one := NewAtomStats()
		one.Seed = seed
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: atomMaxDecisions, MaxTurns: 60, Check: SetAtomCheck(one, atoms)})
		CloseSide(k)
		games++
		st.Add(one)
		if d := res.Div; d != nil && d.What == "atom" {
			fails = append(fails, name+": decision "+strconv.Itoa(d.At)+": "+d.Field+": "+d.Detail)
		}
	}
	c, pool := testCorpus(t)
	cfg := Config{GorgeAutopaySurface: true}
	for _, p := range Pairings(pool, atomPairs, 11) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		k, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
		if err != nil {
			continue
		}
		play("fdn "+p.A+" vs "+p.B, m, p.Seed, k, cfg)
	}
	two := games
	pc, names := cmd5MiniCorpus(t, 40)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	pp := make([]PoolDeck, len(names))
	for i, n := range names {
		pp[i] = PoolDeck{Name: n}
	}
	pod := PodAt(pp, 0, 1, 4)
	m, tbl, err := SetMatchupN(set, pc, pod.Decks(), 0)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, Config{})(m, pod.Seed)
	if err != nil {
		t.Fatal(err)
	}
	play("pod", m, pod.Seed, k, Config{})
	if two == 0 || games == two || st.Decisions == 0 {
		t.Fatalf("precondition: %d two-seat games, %d pod games, %d compared decisions", two, games-two, st.Decisions)
	}
	for _, f := range fails {
		t.Error(f)
	}
	for _, a := range atoms {
		t.Logf("%-24s evals %6d non-empty %6d empty %6d", a.Name, st.Evals[a.Name], st.True[a.Name], st.False[a.Name])
		if st.True[a.Name] == 0 {
			t.Errorf("%s is not proven: no non-empty reading in %d", a.Name, st.Evals[a.Name])
		}
	}
}

// TestAtomDiffSetCatchesAWrongSet: an atom whose kernel program names every
// player where gorge names the opponents diverges, naming the atom.
func TestAtomDiffSetCatchesAWrongSet(t *testing.T) {
	wrong := []SetAtom{{Name: "set:wrong", GorgeDefined: "Opponent", Prog: kop.SetExpr{{Kind: kop.SetEachPlayer}}}}
	c, pool := testCorpus(t)
	cfg := Config{GorgeAutopaySurface: true}
	p := Pairings(pool, 1, 11)[0]
	m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	st := NewAtomStats()
	res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 50, MaxTurns: 60, Check: SetAtomCheck(st, wrong)})
	CloseSide(k)
	if d := res.Div; d == nil || d.What != "atom" || d.Field != "set:wrong" {
		t.Fatalf("the wrong set atom did not diverge: %+v", res.Div)
	}
}
