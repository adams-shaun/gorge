package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestLoseAllStaticKeywordLockstep: a creature under a Continuous override
// static that removes all abilities (Reprobation's "enchanted creature loses
// all abilities") reads no printed keyword to gorge's with<Keyword> words, and
// the kernel must match it. The keyword is Infect, not Toxic: the gate is the
// class, not a Toxic special case. Rot Wolf prints Infect; Reprobation strips
// it. Both name-hash to the same tail pool, so the pool is found by name, never
// fixed (cardset.TailBucketOf).
func TestLoseAllStaticKeywordLockstep(t *testing.T) {
	t.Parallel()

	deck := []cardset.Entry{
		{Name: "Rot Wolf", Count: 8},
		{Name: "Reprobation", Count: 8},
		{Name: "Forest", Count: 12},
		{Name: "Plains", Count: 12},
	}
	bucket := cardset.TailBucketOf("Rot Wolf")
	if cardset.TailBucketOf("Reprobation") != bucket {
		t.Fatalf("Rot Wolf (tail-%02d) and Reprobation (tail-%02d) no longer share a tail pool", bucket, cardset.TailBucketOf("Reprobation"))
	}
	set := cardsettest.Synth(t, fmt.Sprintf("tail-%02d", bucket))
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("tail-%02d does not play the deck: %v", bucket, err)
	}
	reg := synthReg(t)
	var cs [2][]*cards.Card
	for s := range cs {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	// withoutInfect is not a lowered word (kgen refuses it); withInfect alone
	// is read true on every bare Rot Wolf and false on the one Reprobation
	// stripped, so the atom is still proven both ways.
	atoms := []FilterAtom{{Name: "fw:Card.withInfect", GorgeSpec: "Card.withInfect", Ctx: AtomAnyZone}}
	total := NewAtomStats()
	games := 0
	for seed := uint64(1); seed <= 32; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"la-a", "la-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		st := NewAtomStats()
		st.Seed = seed
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: AtomCheckFor(st, atoms, nil)}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		games++
		if d := res.Div; d != nil && d.What == "atom" {
			t.Fatalf("seed %d: diverged at decision %d: %s: %s", seed, d.At, d.Field, d.Detail)
		}
		total.Add(st)
	}
	if games == 0 || total.Decisions == 0 {
		t.Fatalf("%d games, %d compared decisions", games, total.Decisions)
	}
	for _, a := range atoms {
		if total.True[a.Name] == 0 || total.False[a.Name] == 0 {
			t.Fatalf("atom %s is not read both ways by the pool (true %d, false %d): the games never put a creature under the lose-all Aura and beside one not under it", a.Name, total.True[a.Name], total.False[a.Name])
		}
	}
}
