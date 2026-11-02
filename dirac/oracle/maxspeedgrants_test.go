package oracle

import (
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The max-speed grant lockstep: Far Fortune's granted
// plus-1 damage replacement, Racers' Scoreboard's granted ReduceCost and
// Hazoret's "can't attack or block unless you have max speed"
// (PlayerCountPropertyYou$HasPropertyMaxSpeed), against a creature deck so
// the red deck reaches speed 4 in most seeds.

var kspeed2Decks = [2][]cardset.Entry{
	{
		{Name: "Far Fortune, End Boss", Count: 4}, {Name: "Hazoret, Godseeker", Count: 4},
		{Name: "Racers' Scoreboard", Count: 3}, {Name: "Goblin Surveyor", Count: 4},
		{Name: "Hour of Victory", Count: 3}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 16}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6},
	},
}

// kspeed2Tally counts, from the kernel side, the decisions at which a
// controller at speed 4 had each max-speed card on the battlefield.
type kspeed2Tally struct {
	maxSpeed map[string]int // card name: decisions at max speed with it on the battlefield
	games    map[string]int // card name: games in which that happened
}

// kspeed2Policy attacks whenever it can and otherwise casts, so speed rises.
func kspeed2Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	coin := rand.New(rand.NewPCG(seed, 0x5eed2))
	return PolicyFunc(func(c *PickCtx) Canon {
		var attack, play []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActAttack:
				attack = append(attack, a)
			case ActCast, ActPlayLand, ActActivate:
				play = append(play, a)
			}
		}
		switch {
		case len(attack) > 0 && coin.IntN(8) != 0:
			return attack[coin.IntN(len(attack))]
		case len(play) > 0 && coin.IntN(4) != 0:
			return play[coin.IntN(len(play))]
		}
		return rnd.Pick(c)
	})
}

var kspeed2Names = []string{"Far Fortune, End Boss", "Hazoret, Godseeker", "Racers' Scoreboard"}

// kspeed2Games plays seeds 1..n and returns the divergent games, the kernel
// stalls and the tally. With report each divergence or stall fails the test;
// without it the first divergence is logged and ends the run (a mutation).
func kspeed2Games(t *testing.T, n uint64, report bool) (div, stalls int, tally *kspeed2Tally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maxspeedgrants")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kspeed2Decks {
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
	tbl, _, _, err := set.Matchup(kspeed2Decks[0], kspeed2Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally = &kspeed2Tally{maxSpeed: map[string]int{}, games: map[string]int{}}
	var proj dirac.Projection
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"kspeed2-a", "kspeed2-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		check := func(_ *GorgeSide, ks Side) *Divergence {
			r, ok := ks.(*realSide)
			if !ok || (r.s.Speed(0) < 4 && r.s.Speed(1) < 4) {
				return nil
			}
			r.s.Project(&proj)
			for i := 0; i < int(proj.NObj); i++ {
				o := &proj.Objs[i]
				if o.Zone != kop.ZoneBattlefield || r.s.Speed(o.Ctrl) < 4 || int(o.Card) >= len(tbl.Cards) {
					continue
				}
				for _, nm := range kspeed2Names {
					if tbl.Cards[o.Card].Name == nm {
						tally.maxSpeed[nm]++
						seen[nm] = true
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, kspeed2Policy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: check})
		CloseSide(k)
		for nm := range seen {
			tally.games[nm]++
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			msg := "seed " + strconv.FormatUint(seed, 10) + ": diverged at decision " + strconv.Itoa(d.At) + ": " + d.What + " " + d.Field
			if !report {
				t.Log(msg)
				return
			}
			t.Errorf("%s %v %s\nref %v\nker %v", msg, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func kspeed2Seeds(def uint64) uint64 {
	if v := os.Getenv("KSPEED2_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func TestMaxSpeedGrantsLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, tally := kspeed2Games(t, kspeed2Seeds(1000), true)
	t.Logf("maxspeedgrants: %d divergences, %d stalls; decisions at max speed with the card out %v, games %v", div, stalls, tally.maxSpeed, tally.games)
	for _, nm := range kspeed2Names {
		if tally.games[nm] == 0 {
			t.Errorf("no seed reached speed 4 with %s on the battlefield", nm)
		}
	}
}

// TestMaxSpeedGrantsLockstepMutation: the max-speed reads answering "max speed" at
// any speed must diverge (Hazoret attacks early, Far Fortune adds 1 early).
func TestMaxSpeedGrantsLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSpeedGrantsOpen = true
	defer func() { dirac.DebugSpeedGrantsOpen = false }()
	if div, _, _ := kspeed2Games(t, 300, false); div > 0 {
		return
	}
	t.Fatal("no seed diverged with the max-speed reads open: the pool does not exercise them")
}
