package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
)

// caveAskTally counts what the priority-activation policy did.
type caveAskTally struct {
	payAsks    int // unless asks posed to the opponent with a pay option
	oppPays    int // opponent payments chosen
	selfAsks   int // unless asks posed to the activator
	selfPays   int // activator payments chosen
	declines   int // declines chosen
	colourAsks int // colour asks posed
}

// rhysticCavePriorityPolicy is rhysticCavePolicy plus the unless asks: the opponent
// pays {1} whenever it can (so the asked source adds no mana), and the activator pays too
// when selfPays (its own Forest taps for it), else declines.
func rhysticCavePriorityPolicy(seed uint64, selfPays bool, played map[string]int, tl *caveAskTally) Policy {
	inner := rhysticCavePolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if d := c.G.E.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "choosecolor" {
				tl.colourAsks++
			}
			if d := c.G.E.Pending(); d != nil && d.Kind == decision.KModes && d.ResumeKind == "unless_pay" {
				var pay, decline Canon
				for _, o := range c.D.Options {
					if o.Kind() == ActMode && o.Mode() == 0 && len(c.D.Options) == 2 {
						pay = o
					} else {
						decline = o
					}
				}
				if c.D.Player == 1 && len(c.D.Options) == 2 {
					tl.payAsks++
					tl.oppPays++
					return pay
				}
				if c.D.Player == 0 {
					tl.selfAsks++
					if selfPays && len(c.D.Options) == 2 {
						tl.selfPays++
						return pay
					}
				}
				tl.declines++
				if decline != 0 {
					return decline
				}
				return c.D.Options[0]
			}
		}
		return inner.Pick(c)
	})
}

// TestDbManaRhysticCavePriorityActivationLockstep: with Config.PriorityManaAsk the Cave's
// activation is a canonical priority option on both sides, and its colour ask and the
// players' pay/decline asks are canonical decisions (dirac mana_priask.go). The opponent
// holds untapped lands, so it can pay the {1}: the asked Cave then adds no mana. Four seeds
// over the Cave fixture, alternating the gorge autopay surface, 0 divergent.
func TestDbManaRhysticCavePriorityActivationLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var cs [2][]*cards.Card
	var names [2][]string
	for s, d := range rhysticCaveDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
				names[s] = append(names[s], e.Name)
			}
		}
	}
	set := rhysticCaveSet(t, names)
	tbl, _, _, err := set.Matchup(rhysticCaveDecks[0], rhysticCaveDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	at := map[string][]int{}
	for i, n := range names[0] {
		at[n] = append(at[n], i)
	}
	m0, err := NewMatchup(reg, [2]string{"rhystic-a", "rhystic-b"}, cs, 0)
	if err != nil {
		t.Fatal(err)
	}
	m0.Start = &rules.StartSpec{Seats: []rules.StartSeat{
		{Battlefield: []int{at["Forest"][0], at["Rhystic Cave"][0], at["Grizzly Bears"][0]}, Hand: []int{at["Grizzly Bears"][1], at["Trash the Town"][0]}},
		{Battlefield: []int{0, 1, 2}, Hand: []int{3, 4, 5, 6}},
	}}
	played := map[string]int{}
	var tl caveAskTally
	for seed := uint64(1); seed <= 4; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0, PriorityManaAsk: true}
		m := *m0
		k, err := SetSide(set, tbl, cfg)(&m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		res := Lockstep(&m, seed, k, rhysticCavePriorityPolicy(seed, seed%4 < 2, played, &tl), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			var acts []string
			for _, a := range res.Actions[:min(d.At, len(res.Actions))] {
				acts = append(acts, a.String())
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nactions %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, acts)
		}
	}
	if played["Rhystic Cave"] == 0 {
		t.Fatal("the Cave was never activated at priority: its asks are not compared")
	}
	if tl.colourAsks == 0 || tl.oppPays == 0 || tl.selfAsks == 0 || tl.selfPays == 0 {
		t.Fatalf("asks %+v: want a colour ask, the activator's unless ask, its payment and an opponent payment", tl)
	}
	t.Logf("4 games, %d Cave activations, %+v, 0 divergent", played["Rhystic Cave"], tl)
}
