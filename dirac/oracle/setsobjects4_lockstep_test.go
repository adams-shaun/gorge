package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
)

// Dig Choser$ (kgen lower_digparams.go digChoser; the kernel's OpDigEx W
// side input, zone_digtoporder.go digChooser) in lockstep over the
// setsobjects4 fixture pool, at two seats and at four (a Commander pod):
//
//   - Animal Magnetism and Murmurs from Beyond: "an opponent chooses" from
//     the controller's dig. gorge narrows the opponents by the controller's
//     dig_chooser KChoose (asked only with more than one, so only at four
//     seats); the kernel asks ExtChoosePlayerMid and the chosen opponent
//     picks the card.
//   - Ashnod's Cylix: the targeted player picks from their own library.
//   - Gate to the Aether: the upkeep player picks from their own library.
//
// The scenario is reached when the dig's pick is posed to a player other than
// the source's controller (each card at least once over the run) and, at four
// seats, the opponent choice is asked. The mutation drops each dig's W (every
// pick back to the controller) and must diverge.
//
// The expected answer at every decision is gorge's.

var so4Cards = []string{"Animal Magnetism", "Murmurs from Beyond", "Ashnod's Cylix", "Gate to the Aether"}

type so4Tally struct {
	foreign  map[string]int // dig picks posed to a player other than the source's controller, by card
	chooser  int            // dig_chooser opponent asks
	lastSeen string         // the last counted decision, so a re-posed one counts once
}

func so4Policy(seed uint64, tl *so4Tally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			if d := g.E.Pending(); d != nil {
				key := fmt.Sprintf("%d/%d/%d/%s", c.Seq, d.Player, d.Source, d.ResumeKind)
				if key != tl.lastSeen {
					tl.lastSeen = key
					if d.ResumeKind == "dig_chooser" {
						tl.chooser++
					}
					if src := g.E.G.Obj(d.Source); src != nil && src.Face() != nil && d.Kind == decision.KChoose &&
						len(d.Options) > 0 && d.Options[0].Kind == "dig" && d.Player != src.Controller {
						tl.foreign[src.Face().Name]++
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || c.Seq%4 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func runSO4(t *testing.T, seats, seeds int, mutate bool) (fixtureRun, *so4Tally) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("setsobjects4")
	if err != nil {
		t.Fatal(err)
	}
	list := []cardset.Entry{
		{Name: "Animal Magnetism", Count: 6}, {Name: "Murmurs from Beyond", Count: 3}, {Name: "Ashnod's Cylix", Count: 3},
		{Name: "Gate to the Aether", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 11}, {Name: "Island", Count: 9},
	}
	var deck []string
	for _, e := range list {
		for range e.Count {
			deck = append(deck, e.Name)
		}
	}
	lists := make([][]cardset.Entry, seats)
	for i := range lists {
		lists[i] = list
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range so4Cards {
		n := mutateCard(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpDigEx && mutate {
				in.W = kop.RefNone
			}
		})
		if mutate && n == 0 {
			t.Fatalf("%s: no dig W to drop", name)
		}
	}
	tl := &so4Tally{foreign: map[string]int{}}
	var run fixtureRun
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"so4-0", "so4-1"}, Cards: [2][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
		for s := 2; s < seats; s++ {
			r.MoreDecks = append(r.MoreDecks, fmt.Sprintf("so4-%d", s))
			r.MoreCards = append(r.MoreCards, deck)
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 25
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, so4Policy(seed, tl), rc)
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.StallUnmodelled {
			if !mutate {
				t.Fatalf("%d seats, seed %d: kernel stopped: %s", seats, seed, res.Stall)
			}
			run.stalls++
		}
		if d := res.Div; d != nil {
			if !mutate {
				t.Fatalf("%d seats, seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seats, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			run.divs++
			if run.first == "" {
				run.first = fmt.Sprintf("seed %d at %d: %s %s", seed, d.At, d.What, d.Field)
			}
		}
	}
	return run, tl
}

func TestSetsObjects4DigChoserLockstep(t *testing.T) {
	for _, seats := range []int{2, 4} {
		t.Run(fmt.Sprintf("seats%d", seats), func(t *testing.T) {
			run, tl := runSO4(t, seats, 24, false)
			t.Logf("%d seats, %d games: foreign dig picks %v, opponent choices %d", seats, run.games, tl.foreign, tl.chooser)
			for _, name := range so4Cards {
				if tl.foreign[name] == 0 {
					t.Errorf("%d seats: %s's pick was never posed to a player other than its controller", seats, name)
				}
			}
			if seats == 4 && tl.chooser == 0 {
				t.Errorf("4 seats: the dig's opponent choice was never asked")
			}
		})
	}
	t.Run("mutation-W", func(t *testing.T) {
		run, _ := runSO4(t, 2, 16, true)
		run.requireDivergence(t, "every Dig Choser$ pick posed to the controller")
	})
}
