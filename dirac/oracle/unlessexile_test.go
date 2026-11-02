package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ExileFromGrave<N/Spec> as an UnlessCost$ (CR
// 118.12, 118.3). gorge's pay.AdvanceUnless declines with fewer than N
// eligible, takes exactly N unasked and otherwise asks one "exilecost" pick
// (the kernel's ChCostExile; a trigger's own cost is "exile_cost").
// Trystan's back face and Heated Argument pay UnlessSwitched, Rotting Giant
// UnlessPaid.
var kunlessexileDecks = [2][]cardset.Entry{
	{
		{Name: "Trystan, Callous Cultivator", Count: 4}, {Name: "Rotting Giant", Count: 4}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Elvish Mystic", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Heated Argument", Count: 6}, {Name: "Thought Scour", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Island", Count: 10},
	},
}

// kunlessexileTally counts unless exile payments from gorge's decisions:
// EU+ paid with a pick asked, EU= paid with exactly N eligible (unasked),
// EU- declined (or posed with no way to pay).
type kunlessexileTally struct {
	games, plus, eq, minus int
}

// kunlessexilePolicy pays or declines each unless ask by seed and decision
// parity and aims Thought Scour at its caster; everything else is random.
func kunlessexilePolicy(seed uint64, tal *kunlessexileTally) Policy {
	rnd := RandomPolicy(seed)
	awaitPay := false
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		d := c.G.E.Pending()
		if d == nil {
			return rnd.Pick(c)
		}
		if awaitPay {
			awaitPay = false
			if tal != nil {
				if d.ResumeKind == "unless_cost" {
					tal.plus++
				} else {
					tal.eq++
				}
			}
		}
		if d.ResumeKind == "unless_pay" {
			want := "unless_decline"
			if (seed+uint64(c.Seq))%2 == 0 {
				want = "unless_pay"
			}
			for i, o := range d.Options {
				if o.Mode != want {
					continue
				}
				for _, a := range c.D.Options {
					if a.Kind() == ActMode && int(a.Mode()) == i {
						if want == "unless_pay" {
							awaitPay = true
						} else if tal != nil {
							tal.minus++
						}
						return a
					}
				}
			}
			if tal != nil {
				tal.minus++ // posed with no way to pay
			}
		}
		if o := c.G.E.G.Obj(d.Source); o != nil && o.Face() != nil && o.Face().Name == "Thought Scour" {
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget && a.Tgt() == RefPlayer+Ref(d.Player) {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// kunlessexileRun plays seeds 1..n, failing on each divergence (report) or
// stopping at the first; it returns the number of divergent games.
func kunlessexileRun(t *testing.T, n uint64, report bool, tal *kunlessexileTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlessexile")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kunlessexileDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kunlessexileDecks[0], kunlessexileDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ue-a", "ue-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kunlessexilePolicy(seed, tal), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if tal != nil {
			tal.games++
		}
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

func TestUnlessexileLockstep(t *testing.T) {
	t.Parallel()
	var tal kunlessexileTally
	div := kunlessexileRun(t, 200, true, &tal)
	t.Logf("%d games, %d divergences: EU+ %d, EU= %d, EU- %d", tal.games, div, tal.plus, tal.eq, tal.minus)
	if tal.plus < 10 || tal.eq < 10 || tal.minus < 10 {
		t.Errorf("unless exile payments: EU+ %d, EU= %d, EU- %d; want at least 10 each", tal.plus, tal.eq, tal.minus)
	}
}
