package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestStorageXPaysSpellLockstep plays the storage-battery probe: Bottomless
// Vault and Blightsoil Druid ({1}{B}) against Plains. The Vault's only mana
// ability is its storage X one, so every cast of the Druid is paid by
// counters; on the odd seeds (gorge pays the scripted prefix) both engines
// must cast it, and
// no seed may diverge. The even seeds run the autopay surface, which drops
// scripted casts on both sides, so they are evidence for the no-divergence
// half only.
func TestStorageXPaysSpellLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const vault = "Bottomless Vault"
	const spell = "Blightsoil Druid"

	// synthReg skips the test when the corpus is absent (a fresh clone);
	// printedDelveSet would fail there instead.
	reg := synthReg(t)
	decks := [2][]cardset.Entry{
		{{Name: vault, Count: 30}, {Name: spell, Count: 10}},
		{{Name: "Plains", Count: 40}},
	}
	// The table is built for exactly these cards: the synthetic pools are
	// re-dealt as lowering and refusal change, so no pool is guaranteed to
	// hold both.
	set := printedDelveSet(t, [2][]string{{vault, spell}, {"Plains"}})
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}

	for seed := uint64(1); seed <= 10; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		casts := 0
		for _, a := range res.Actions {
			if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == spell {
				casts++
			}
		}
		t.Logf("seed %d: %s cast %d times", seed, spell, casts)
		// The odd seeds are the scripted-prefix surface: a game where the
		// Druid is never cast proves nothing about the Vault paying it.
		if seed%2 == 1 && casts == 0 {
			t.Errorf("seed %d: %s is never cast; the Vault's X ability never paid it", seed, spell)
		}
	}
}
