package oracle

import (
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC combat-6 family in lockstep against gorge, one deck pair per
// spelling kgen now lowers (pool dirac/cardset/pools/fixtures/mbccombat6):
//
//   - an Effect Mode$ CombatDamageToughness over the targeted creature
//     (Plagon, Lord of the Beach): CR 510.1, kernel ExtAsgnGrant C 0.
//   - an Effect Mode$ AssignCombatDamageAsUnblocked with no Optional$ over X
//     targeted blocked creatures (Outmaneuver): CR 510.1c-d, ExtAsgnGrant C 1,
//     gorge's mandatory arm (asUnblockedNeeding auto-accepts, no election).
//   - Karlach, Fury of Avernus's FirstCombat$ on an AttackersDeclared trigger
//     (kgen TestFirstCombatAttackersDeclared): the card also needs Choose a
//     Background, so it has no deck here.
//   - Tromokratis's Min$ All is covered by tromokratis_minall_test.go.
//
// Each subtest first asserts the deck is playable, then plays the lockstep
// games and asserts the kernel's counter for the mechanism moved, so a pool
// that never reaches the mechanism cannot pass; with the lowering reverted the
// card is refused and the assertion fails.

const mbcCombat6Pool = "mbccombat6"

var mbcPlagonDecks = [2][]cardset.Entry{
	{
		{Name: "Plagon, Lord of the Beach", Count: 6}, {Name: "Wall of Wood", Count: 6},
		{Name: "Wall of Air", Count: 4}, {Name: "Wall of Stone", Count: 2},
		{Name: "Island", Count: 14}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Craw Wurm", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

var mbcOutmaneuverDecks = [2][]cardset.Entry{
	{
		{Name: "Outmaneuver", Count: 6}, {Name: "Hill Giant", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Craw Wurm", Count: 2},
		{Name: "Mountain", Count: 18},
	},
	{
		{Name: "Wall of Wood", Count: 6}, {Name: "Wall of Air", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 11}, {Name: "Island", Count: 11},
	},
}

var mbcCombat6Decks = []struct {
	card     string
	decks    [2][]cardset.Entry
	deltaOf  func() int64
	minDelta int64
	what     string
}{
	{"Plagon, Lord of the Beach", mbcPlagonDecks, func() int64 { return dirac.AsgnGrantStats.Tough.Load() }, 3,
		"combat damage amounts read by toughness from an ExtAsgnGrant record"},
	{"Outmaneuver", mbcOutmaneuverDecks, func() int64 { return dirac.AsgnGrantStats.Unblocked.Load() }, 3,
		"blocked attackers assigned as though unblocked by an ExtAsgnGrant record"},
}

func TestMBCCombat6Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcCombat6Decks {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			before := tc.deltaOf()
			combat6Play(t, tc.card, tc.decks)
			if n := tc.deltaOf() - before; n < tc.minDelta {
				t.Fatalf("%s: %d (< %d): the pool never reaches the mechanism", tc.what, n, tc.minDelta)
			} else {
				t.Logf("%s: %d", tc.what, n)
			}
		})
	}
}

// TestMBCCombat6DecksPlayable asserts each family card is playable in the
// pool's set on its own, so a refused card is named here and not only by a
// stall far into the games.
func TestMBCCombat6DecksPlayable(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcCombat6Decks {
		_, set, gd := lockstepPool(t, mbcCombat6Pool, tc.decks)
		for s, d := range gd {
			if !SetDeckSupported(set, d) {
				t.Errorf("seat %d: %s not playable in the %s pool", s, tc.card, mbcCombat6Pool)
			}
		}
	}
}

// combat6Gate is castFirstPolicy withholding the family card's cast or
// activation until the declare blockers step: castFirstPolicy alone casts
// Outmaneuver in the first window it is offered, before any creature is
// blocked, and activates Plagon's ability with no creature in combat.
type combat6Gate struct {
	m    *Matchup
	card string
	base Policy
	gs   *GorgeSide
}

func (g *combat6Gate) pick(c *PickCtx) Canon {
	if c.G != nil {
		g.gs = c.G
	}
	if g.gs != nil && g.gs.E.G.Step == state.StepDeclareBlockers {
		return g.base.Pick(c)
	}
	d := *c.D
	d.Options = nil
	for _, o := range c.D.Options {
		if k := o.Kind(); (k == ActCast || k == ActActivate) && g.m.InstanceCard(int(o.Src())) == g.card {
			continue
		}
		d.Options = append(d.Options, o)
	}
	if len(d.Options) == 0 {
		return g.base.Pick(c)
	}
	return g.base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
}

// combat6Play plays the deck pair in lockstep over the seeds, the family
// card gated to the declare blockers step on three seeds in four (the rest
// play the plain castFirstPolicy and a random policy, so the early casts
// stay covered), failing on any divergence or unmodelled stall.
func combat6Play(t *testing.T, card string, decks [2][]cardset.Entry) {
	t.Helper()
	seeds := uint64(400)
	if v := os.Getenv("KMBCCOMBAT6_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	reg, set, gd := lockstepPool(t, mbcCombat6Pool, decks)
	clean := 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		tbl, _, _, err := set.Matchup(decks[0], decks[1])
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for seed := lo; seed <= hi; seed++ {
			m, err := NewMatchup(reg, [2]string{"combat6-a", "combat6-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castFirstPolicy(seed)
			if seed%8 == 0 {
				pol = RandomPolicy(seed)
			} else if seed%4 != 1 {
				pol = PolicyFunc((&combat6Gate{m: m, card: card, base: pol}).pick)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				from := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				continue
			}
			n++
		}
		mu.Lock()
		clean += n
		mu.Unlock()
	})
	t.Logf("%s: %d seeds, %d clean", card, seeds, clean)
}
