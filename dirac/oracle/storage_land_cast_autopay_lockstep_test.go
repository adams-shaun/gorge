package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestStorageLandCastAutopayReprosLockstep plays Fountain of Cho (a storage
// battery) with three Islands, Bastion Inventor ({5}{U}, improvise) and cheap
// artifacts on the autopay surface, where gorge's cast payment plan announces
// no SubCounter<X>. The policy banks counters and leaves some batteries
// untapped, so the generic part of a cast is a choice between the battery, the
// Islands and improvise. No seed may diverge: the kernel's cast payment may not
// tap the land for all its counters where gorge leaves it untapped (tail-67
// pair 1, decision 483, on the branch before 1e9beeedc4). The test asserts its
// own preconditions: some cast spends the battery's counters, else a pass
// would prove nothing.
func TestStorageLandCastAutopayReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	const land, spell, basic = "Fountain of Cho", "Bastion Inventor", "Island"
	reg := synthReg(t)
	decks := [2][]cardset.Entry{
		{{Name: land, Count: 15}, {Name: basic, Count: 3}, {Name: spell, Count: 12}, {Name: "Ornithopter", Count: 6}, {Name: "Paradox Engine", Count: 4}},
		{{Name: "Plains", Count: 40}},
	}
	set := printedDelveSet(t, [2][]string{{land, spell, basic, "Ornithopter", "Paradox Engine"}, {"Plains"}})
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
	const seeds = 120
	cfg := Config{GorgeAutopaySurface: true}
	totalCasts, spent := 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, storageBankPolicy(m, seed, land, spell, &spent), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		for _, a := range res.Actions {
			if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == spell {
				totalCasts++
			}
		}
	}
	t.Logf("%s cast %d times over %d seeds; the battery lost counters %d times", spell, totalCasts, seeds, spent)
	// The preconditions the lockstep result depends on: the spell is cast, and
	// gorge's payment spends the battery's counters in some of those casts.
	if totalCasts == 0 {
		t.Errorf("%s is never cast; the fixture proves nothing", spell)
	}
	if spent == 0 {
		t.Errorf("no cast ever spent %s's storage counters; the fixture never reaches the battery", land)
	}
}

// storageBankPolicy banks storage counters before it casts: it plays lands,
// puts a counter on every battery it can, and offers the spell only once
// banked counters could be spent, so the cast's payment is a real choice
// between the battery and the Islands. Every other pick is seeded random.
func storageBankPolicy(m *Matchup, seed uint64, land, spell string, spent *int) Policy {
	rnd := rand.New(rand.NewPCG(seed, 0x62616e6b)) // "bank"
	banked := 0
	prev := int32(0)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.Game()
			sum := int32(0)
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					if o := g.Obj(id); o.Face().Name == land {
						sum += o.Counter("STORAGE")
					}
				}
			}
			if sum < prev {
				*spent++
			}
			prev = sum
		}
		var lands, puts, rest []Canon
		for _, a := range c.D.Options {
			name := m.InstanceCard(int(a.Src()))
			switch {
			case a.Kind() == ActPlayLand:
				lands = append(lands, a)
			case a.Kind() == ActActivate && name == land:
				puts = append(puts, a)
			case a.Kind() == ActCast && name == spell && banked < 8:
			default:
				rest = append(rest, a)
			}
		}
		switch {
		case len(lands) > 0:
			return lands[0]
		case len(puts) > 0 && rnd.IntN(4) > 0:
			banked++
			return puts[0]
		case len(rest) > 0:
			return rest[rnd.IntN(len(rest))]
		}
		return c.D.Options[0]
	})
}
