package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// UnlessResolveSubs$ WhenPaid / WhenNotPaid (CR 118.12;
// gorge unlessSubsRun) gates the unless node's SubAbility$ chain on the pay
// outcome (Don't Make a Sound and Divert Disaster counter unless {2} is
// paid, and then surveil / make a Lander; Lord Skitter's Butcher's scry
// mode), and a triggered GenericChoice offers its unless-cost branches
// payable or not (Reckless Detective, K'un-Lun Warrior: gorge's placement
// ask filters only targets and ChoiceRestriction).
var kunlesssubsDecks = [2][]cardset.Entry{
	{
		{Name: "Don't Make a Sound", Count: 4}, {Name: "Divert Disaster", Count: 4}, {Name: "Reckless Detective", Count: 3},
		{Name: "K'un-Lun Warrior", Count: 3}, {Name: "Lord Skitter's Butcher", Count: 2}, {Name: "Callous Inspector", Count: 3},
		{Name: "Ornithopter", Count: 3}, {Name: "Bonesplitter", Count: 2}, {Name: "Thought Scour", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Reckless Detective", Count: 3}, {Name: "K'un-Lun Warrior", Count: 3}, {Name: "Lord Skitter's Butcher", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Heated Argument", Count: 2},
		{Name: "Callous Inspector", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Walking Corpse", Count: 4},
		{Name: "Raging Goblin", Count: 3}, {Name: "Bake into a Pie", Count: 1}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
	},
}

// kunlesssubsTally counts the unless asks by card: paid / declined for the
// two counterspells and the Butcher's scry, and branches whose unless ask
// had no way to pay (Detective and K'un-Lun: decline-only).
type kunlesssubsTally struct {
	games, counterPaid, counterDeclined, scryPaid, scryDeclined, unpayable int
}

func kunlesssubsPolicy(seed uint64, tal *kunlesssubsTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		d := c.G.E.Pending()
		if d == nil || d.ResumeKind != "unless_pay" {
			return rnd.Pick(c)
		}
		name := ""
		if o := c.G.E.G.Obj(d.Source); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
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
					if tal != nil {
						switch {
						case name == "Don't Make a Sound" || name == "Divert Disaster":
							if want == "unless_pay" {
								tal.counterPaid++
							} else {
								tal.counterDeclined++
							}
						case name == "Lord Skitter's Butcher":
							if want == "unless_pay" {
								tal.scryPaid++
							} else {
								tal.scryDeclined++
							}
						}
					}
					return a
				}
			}
		}
		if tal != nil {
			switch name {
			case "Reckless Detective", "K'un-Lun Warrior":
				tal.unpayable++
			case "Don't Make a Sound", "Divert Disaster":
				tal.counterDeclined++
			case "Lord Skitter's Butcher":
				tal.scryDeclined++
			}
		}
		return rnd.Pick(c)
	})
}

// kunlesssubsRun plays seeds 1..n; mut patches the generated table.
func kunlesssubsRun(t *testing.T, n uint64, report bool, tal *kunlesssubsTally, mut func(*kop.Table)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlesssubs")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kunlesssubsDecks {
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
	tbl, _, _, err := set.Matchup(kunlesssubsDecks[0], kunlesssubsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	div := 0
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"us-a", "us-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kunlesssubsPolicy(seed, tal), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
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
				return div
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

func TestUnlesssubsLockstep(t *testing.T) {
	t.Parallel()
	var tal kunlesssubsTally
	div := kunlesssubsRun(t, 256, true, &tal, nil)
	t.Logf("%d games, %d divergences: counter paid %d declined %d, scry paid %d declined %d, unpayable branches %d",
		tal.games, div, tal.counterPaid, tal.counterDeclined, tal.scryPaid, tal.scryDeclined, tal.unpayable)
	if tal.counterPaid < 20 || tal.counterDeclined < 20 || tal.scryPaid < 10 || tal.scryDeclined < 5 || tal.unpayable < 10 {
		t.Errorf("unless asks: counter paid %d declined %d, scry paid %d declined %d, unpayable %d; want at least 20, 20, 10, 5, 10",
			tal.counterPaid, tal.counterDeclined, tal.scryPaid, tal.scryDeclined, tal.unpayable)
	}
}

// TestUnlesssubsMutation proves the pool exercises the sub gate: with the
// jump over Don't Make a Sound's subs removed (skip 0), the subs run on both
// branches and the lockstep diverges.
func TestUnlesssubsMutation(t *testing.T) {
	t.Parallel()
	patched := 0
	div := kunlesssubsRun(t, 128, false, nil, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name != "Don't Make a Sound" {
				continue
			}
			for j := range tbl.Cards[i].Abilities {
				for k := range tbl.Cards[i].Abilities[j].Prog {
					if in := &tbl.Cards[i].Abilities[j].Prog[k]; in.Op == kop.OpSkipUnless {
						in.A = kop.Const(0)
						patched++
					}
				}
			}
		}
	})
	if patched != 1 {
		t.Fatalf("patched %d jumps in Don't Make a Sound, want 1", patched)
	}
	if div == 0 {
		t.Fatal("no seed diverged with the sub gate's jump removed: the pool never pays or declines a counterspell's unless")
	}
}
