package oracle

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC tokens-2 family in lockstep against gorge (pool
// dirac/cardset/pools/fixtures/mbctokens2):
//
//   - a Token with TokenPower$/TokenToughness$ and a TokenAmount$ of X
//     (Gelatinous Genesis's "create X X/X Ooze tokens"): gorge resolves both
//     sides once, mints X tokens and sets each (effects/token.go), so the
//     kernel's ExtTokenPT sets every token of the mint, not only the first.
//   - a comma-listed TokenScript$ with TokenOwner$ You (Specimen Collector's
//     Squirrel and Crab; Fae Offering's Clue, Food and Treasure at an end
//     step after a creature and a noncreature spell).
//
// The test first asserts both decks are playable, then plays the lockstep
// games and asserts, from gorge's side, that each mechanism happened, so a
// pool that never reaches it cannot pass; with the lowering reverted the
// cards are refused (or the multi-token mint stalls) and the test fails.

const mbcTokens2Pool = "mbctokens2"

var mbcTokens2Decks = [2][]cardset.Entry{
	{
		{Name: "Gelatinous Genesis", Count: 5}, {Name: "Specimen Collector", Count: 5},
		{Name: "Fae Offering", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Giant Growth", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Murder", Count: 4},
		{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11},
	},
}

// mbcTokens2Tally counts, from gorge's side, the tokens each card minted.
type mbcTokens2Tally struct {
	byName    map[string]int
	multiOoze int // decisions after which two or more new Oozes had appeared (an X mint)
	badOoze   []string
	objsAt    int
}

func (p *mbcTokens2Tally) reset() { p.objsAt = 0 }

func (p *mbcTokens2Tally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if len(g.Objs) < p.objsAt {
		p.objsAt = 0
	}
	oozes := 0
	for i := p.objsAt; i < len(g.Objs); i++ {
		ob := &g.Objs[i]
		if !ob.IsToken || ob.Face() == nil {
			continue
		}
		n := ob.Face().Name
		p.byName[n]++
		if n == "Ooze Token" {
			oozes++
		}
	}
	p.objsAt = len(g.Objs)
	if oozes >= 2 {
		p.multiOoze++
	}
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.IsToken && ob.Zone == state.ZBattlefield && ob.Face() != nil && ob.Face().Name == "Ooze Token" {
			id := state.ObjID(i + 1)
			if pw, tg := ref.E.Power(id), ref.E.Toughness(id); pw != tg && len(p.badOoze) < 5 {
				p.badOoze = append(p.badOoze, "Ooze token not X/X")
			}
		}
	}
	return nil
}

func TestMBCTokens2DecksPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, mbcTokens2Pool, mbcTokens2Decks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: the tokens-2 cards are not playable in the %s pool", s, mbcTokens2Pool)
		}
	}
}

func TestMBCTokens2Lowers(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, mbcTokens2Pool, mbcTokens2Decks)
	tbl, _, _, err := set.Matchup(mbcTokens2Decks[0], mbcTokens2Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	tal := mbcTokens2Tally{byName: map[string]int{}}
	const seeds = 192
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"tok2-a", "tok2-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tal.reset()
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tal.check})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	for _, b := range tal.badOoze {
		t.Error(b)
	}
	t.Logf("%d games, %d divergences: tokens %v, %d X mints of two or more", seeds, div, tal.byName, tal.multiOoze)
	for _, want := range []struct {
		name string
		min  int
	}{{"Ooze Token", 10}, {"Squirrel Token", 10}, {"Crab Token", 10}, {"Clue Token", 3}, {"Food Token", 3}, {"Treasure Token", 3}} {
		if n := tal.byName[want.name]; n < want.min {
			t.Errorf("%s: %d minted (< %d): the pool never reaches the mechanism", want.name, n, want.min)
		}
	}
	if tal.multiOoze < 3 {
		t.Errorf("%d X mints of two or more Oozes (< 3)", tal.multiOoze)
	}
}

// TestMBCTokens2ListAdderRefused: Fae Offering's comma-listed mint beside
// Jolene, the Plunder Queen's Treasure adder is refused at the matchup.
// gorge appends the adder's one extra token after the whole creation event
// (rules/replacement_token.go applyTokenReplacementToPlan); the kernel mints
// the list op by op, so a game would reach the unmodelled tokrewrite stall
// (repl_tokorder.go replTokensOp). Each half alone still builds.
func TestMBCTokens2ListAdderRefused(t *testing.T) {
	t.Parallel()
	_, set, _ := lockstepPool(t, mbcTokens2Pool, mbcTokens2Decks)
	deck := func(card, land string) []cardset.Entry {
		return []cardset.Entry{{Name: card, Count: 10}, {Name: land, Count: 30}}
	}
	fae, jolene, lands := deck("Fae Offering", "Forest"), deck("Jolene, the Plunder Queen", "Mountain"), deck("Island", "Swamp")
	for _, ok := range [][2][]cardset.Entry{{fae, lands}, {jolene, lands}} {
		if _, _, err := set.MatchupN([][]cardset.Entry{ok[0], ok[1]}); err != nil {
			t.Fatalf("precondition: one half alone must build: %v", err)
		}
	}
	_, _, err := set.MatchupN([][]cardset.Entry{fae, jolene})
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != "tokrewrite" {
		t.Fatalf("Fae Offering beside Jolene, the Plunder Queen: got %v, want the tokrewrite interaction refusal", err)
	}
	if !slices.Contains(ir.Cards, "Fae Offering") || !slices.Contains(ir.Cards, "Jolene, the Plunder Queen") {
		t.Fatalf("refusal names %v, want both halves", ir.Cards)
	}
}
