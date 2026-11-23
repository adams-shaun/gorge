package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The zone, printed-layout and base-P/T filter words (T13c). The three seed
// sets TestAtomCoverage sums read them all both ways but Card.DoubleFaced:
// no deck there holds a transforming card, so the seed sets' reads of the
// atom are all false. It is proven here, over the facestransform pool whose
// games are full of transforming cards, under the same atom check.

// zoneProvAtoms is the T13c zone-prov atom list, the atoms
// atomdiff_fw_zoneprov.go registers (the same Ctx each: TopLibrary reads
// only off the battlefield and stack, the printed layout and the base P/T
// compares everywhere but the stack).
func zoneProvAtoms() []FilterAtom {
	anyButStack := AtomAnyZone &^ AtomStack
	var out []FilterAtom
	for _, a := range []FilterAtom{
		{Name: "fw:Card.TopLibrary", GorgeSpec: "Card.TopLibrary", Ctx: AtomAnyZone &^ (AtomBattlefield | AtomStack)},
		{Name: "fw:Card.DoubleFaced", GorgeSpec: "Card.DoubleFaced", Ctx: anyButStack},
		{Name: "fw:Card.basePowerGE2", GorgeSpec: "Card.basePowerGE2", Ctx: anyButStack},
		{Name: "fw:Card.basePowerEQ1", GorgeSpec: "Card.basePowerEQ1", Ctx: anyButStack},
		{Name: "fw:Card.baseToughnessLE3", GorgeSpec: "Card.baseToughnessLE3", Ctx: anyButStack},
		{Name: "fw:Card.inRealZoneBattlefield", GorgeSpec: "Card.inRealZoneBattlefield", Ctx: AtomAnyZone},
		{Name: "fw:Card.inRealZoneGraveyard", GorgeSpec: "Card.inRealZoneGraveyard", Ctx: AtomAnyZone},
		{Name: "fw:Card.inRealZone Graveyard", GorgeSpec: "Card.inRealZone Graveyard", Ctx: AtomAnyZone},
	} {
		out = append(out, a)
	}
	return out
}

// zoneProvSagas is the saga deck of facestransform_test.go's first matchup:
// chapter III exiles and returns the Sagas transformed, so a battlefield, a
// library, a graveyard and exile all hold double-faced cards.
var zoneProvSagas = []cardset.Entry{
	{Name: "Befriending the Moths", Count: 2}, {Name: "Boseiju Reaches Skyward", Count: 2},
	{Name: "Era of Enlightenment", Count: 4},
	{Name: "Michiko's Reign of Truth", Count: 2}, {Name: "Okiba Reckoner Raid", Count: 2},
	{Name: "Tales of Master Seshiro", Count: 2}, {Name: "Teachings of the Kirin", Count: 2},
	{Name: "The Modern Age", Count: 2}, {Name: "The Restoration of Eiganjo", Count: 2},
	{Name: "The Shattered States Era", Count: 2},
	{Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
	{Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
}

// TestAtomDiffZoneProvFocus: the zone-prov atoms agree between gorge's
// matcher and the kernel over facestransform pairs (two pairs, both
// surfaces) and one cmd5 pod, and Card.DoubleFaced -- the one the seed sets
// cannot read true -- is read both ways by the pairs.
func TestAtomDiffZoneProvFocus(t *testing.T) {
	atoms := zoneProvAtoms()
	total := NewAtomStats()
	games := 0
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t)) // skips first: a fresh clone has no .cards
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("facestransform")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(zoneProvSagas, zoneProvSagas)
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for _, e := range zoneProvSagas {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			decks[0] = append(decks[0], c)
			decks[1] = append(decks[1], c)
		}
	}
	for surface := 0; surface < 2; surface++ {
		cfg := Config{GorgeAutopaySurface: surface == 0}
		for seed := uint64(1); seed <= 2; seed++ { // two pairs per surface
			m, err := NewMatchup(reg, [2]string{"kzoneprov-a", "kzoneprov-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			st := NewAtomStats()
			st.Seed = seed
			res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: atomMaxDecisions, MaxTurns: 60, Check: AtomCheckFor(st, atoms, nil)})
			CloseSide(k)
			games++
			total.Add(st)
			if d := res.Div; d != nil && d.What == "atom" {
				t.Errorf("surface %d seed %d: decision %d: %s: %s", surface, seed, d.At, d.Field, d.Detail)
			}
		}
	}
	// One pod: the cmd5 mini corpus, the zone-prov atoms compared there too
	// (a four-seat game reaches battlefield, graveyard and library
	// candidates the pairs do not).
	c, names := cmd5MiniCorpus(t, 40)
	set5, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set5 = cardset.AllowUnverified(set5)
	pool := make([]PoolDeck, len(names))
	for i, n := range names {
		pool[i] = PoolDeck{Name: n}
	}
	p := PodAt(pool, 0, 1, 4)
	m, tbl5, err := SetMatchupN(set5, c, p.Decks(), 0)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	k, err := SetSide(set5, tbl5, cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	st := NewAtomStats()
	st.Seed = p.Seed
	res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: atomMaxDecisions, MaxTurns: 60, Check: AtomCheckFor(st, atoms, nil)})
	CloseSide(k)
	games++
	total.Add(st)
	if d := res.Div; d != nil && d.What == "atom" {
		t.Errorf("pod: decision %d: %s: %s", d.At, d.Field, d.Detail)
	}
	if games == 0 || total.Decisions == 0 {
		t.Fatalf("%d games, %d compared decisions", games, total.Decisions)
	}
	var unproven []string
	for _, a := range atoms {
		if total.True[a.Name] == 0 || total.False[a.Name] == 0 {
			unproven = append(unproven, a.Name)
		}
	}
	sort.Strings(unproven)
	for _, n := range unproven {
		if n == "fw:Card.DoubleFaced" {
			continue // asserted on its own below
		}
		t.Errorf("atom %s is not read both ways by the focus seed", n)
	}
	if total.True["fw:Card.DoubleFaced"] == 0 || total.False["fw:Card.DoubleFaced"] == 0 {
		t.Fatalf("fw:Card.DoubleFaced is not read both ways (true %d, false %d): the seed holds no transforming card the check reaches", total.True["fw:Card.DoubleFaced"], total.False["fw:Card.DoubleFaced"])
	}
}
