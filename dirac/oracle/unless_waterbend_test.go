package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The unless-waterbend lockstep: Waterbending Lesson's DB$ Discard
// UnlessCost$ Waterbend<2> (gorge pay.AdvanceUnless: pay / decline, then the
// taps owed past the floating pool, unasked with none or exactly that many
// candidates). The expected answer at every decision is gorge's.
var kunlesswbDecks = [2][]cardset.Entry{
	{
		{Name: "Waterbending Lesson", Count: 4}, {Name: "The Unagi of Kyoshi Island", Count: 3}, {Name: "Ornithopter", Count: 5},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 2}, {Name: "Island", Count: 11}, {Name: "Forest", Count: 4},
		{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Ornithopter", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 4}, {Name: "Island", Count: 6},
	},
}

// kunlesswbPolicy answers Lesson's pay / decline asks pay, pay, decline in
// turn and counts the arms; the rest is cast-first (seed%8 == 0: random).
func kunlesswbPolicy(seed uint64, counts map[string]int, flip *int) Policy {
	base := castFirstPolicy(seed)
	if seed%8 == 0 {
		base = RandomPolicy(seed)
	}
	rnd := RandomPolicy(seed ^ 0x5eed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		d := c.G.Pending()
		if d == nil {
			return base.Pick(c)
		}
		if d.ResumeKind == "unless_cost" || d.ResumeKind == "ward_waterbend" {
			counts["pick/"+d.ResumeKind]++
			return base.Pick(c)
		}
		if d.ResumeKind != "unless_pay" {
			return base.Pick(c)
		}
		name := ""
		if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		arm := map[string]string{"Waterbending Lesson": "lesson", "The Unagi of Kyoshi Island": "ward"}[name]
		if strings.HasSuffix(d.Prompt, " for ward?") {
			arm = "ward" // no other ward card is in these decks
		}
		if arm == "" {
			return base.Pick(c)
		}
		pay := *flip%3 != 2
		*flip++
		want := decision.ModeUnlessDecline
		if pay {
			want = decision.ModeUnlessPay
		}
		idx := -1
		for _, o := range d.Options {
			if o.Mode == want {
				idx = o.Index
			}
		}
		for _, a := range c.D.Options {
			if idx < 0 || a.Kind() != ActMode || int(a.Mode()) != idx {
				continue
			}
			if pay {
				counts[arm+"/paid"]++
			} else {
				counts[arm+"/declined"]++
			}
			return a
		}
		return rnd.Pick(c)
	})
}

func TestUnlessWaterbendLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlesswaterbend")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kunlesswbDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kunlesswbDecks[0], kunlesswbDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	flip := 0
	for seed := uint64(1); seed <= 300; seed++ {
		m, err := NewMatchup(reg, [2]string{"kunlesswb-a", "kunlesswb-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kunlesswbPolicy(seed, counts, &flip), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			counts["stall"]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("counts %v", counts)
	for arm, min := range map[string]int{"lesson/paid": 5, "lesson/declined": 3, "pick/unless_cost": 3,
		"ward/paid": 3, "ward/declined": 3, "pick/ward_waterbend": 2} {
		if counts[arm] < min {
			t.Errorf("%s: %d, want at least %d", arm, counts[arm], min)
		}
	}
	if counts["stall"] != 0 {
		t.Errorf("%d stalled games", counts["stall"])
	}
}
