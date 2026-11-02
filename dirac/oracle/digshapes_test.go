package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// digshapes pool, whose carriers are DigUntil effects putting the found card
// onto the battlefield or reading a descriptive key:
//
//   - Identity Echo: Valid$ Creature,Planeswalker, FoundDestination$
//     Battlefield (a non-land, non-Aura Valid$ enters plainly);
//   - Raph & Mikey, Troublemakers: the found creature enters Tapped$ and
//     Attacking$ True (gorge effects/diguntil.go:176-184, kop.DuFoundAttacking);
//
//   - Solstice Revelations: NoPutDesc$ (descriptive) and a Play tail whose plain
//     bounce must not move the picked free cast out of exile (gorge begins the
//     cast at the answer).
//
// The decks hold enchantments (Identity Echo) and lands so the scan skips
// non-matches.

var kdsDecks = [2][]cardset.Entry{
	{
		{Name: "Identity Echo", Count: 4}, {Name: "Raph & Mikey, Troublemakers", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Mineshaft Spider", Count: 3}, {Name: "Lasyd Prowler", Count: 3}, {Name: "Rainveil Rejuvenator", Count: 2},
		{Name: "Patchwork Beastie", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Solstice Revelations", Count: 1}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Raph & Mikey, Troublemakers", Count: 3}, {Name: "Identity Echo", Count: 2},
		{Name: "Lasyd Prowler", Count: 3}, {Name: "Mountain", Count: 11}, {Name: "Forest", Count: 7},
	},
}

const kdsMin = 20

// kdsScan counts the mechanics in gorge's event log: the creature put onto the
// battlefield by Identity Echo's DigUntil and the creature Raph & Mikey's
// trigger puts onto the battlefield attacking.
func kdsScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	face := func(id state.ObjID) []string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return append([]string{ob.Face().Name}, ob.Face().Types...)
		}
		return nil
	}
	var trig string // the source of the latest trigger or ability pushed
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush, events.AbilityPush:
			if f := face(ev.Obj); len(f) > 0 {
				trig = f[0]
			}
		case events.TokenAttacks:
			if ev.Text == "entered attacking" && trig == "Raph & Mikey, Troublemakers" {
				n["raph"]++
			}
		case events.MoveZone:
			f := face(ev.Obj)
			if len(f) == 0 {
				continue
			}
			isType := func(ts ...string) bool {
				for _, t := range f[1:] {
					for _, w := range ts {
						if t == w {
							return true
						}
					}
				}
				return false
			}
			switch {
			case ev.From == state.ZLibrary && ev.To == state.ZBattlefield && trig == "Identity Echo" && isType("Creature", "Planeswalker"):
				n["echo"]++
			}
		}
	}
}

// runDigshapes plays the seeded games; mutate clears kop.DuFoundAttacking
// from Raph & Mikey's DigUntil in the generated table and stops at the first
// divergence.
func runDigshapes(t *testing.T, seeds uint64, mutate bool) (games, divs int, played map[string]int, branch map[string]int, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("digshapes")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdsDecks {
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
	tbl, _, _, err := set.Matchup(kdsDecks[0], kdsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		cleared := 0
		for ci := range tbl.Cards {
			if tbl.Cards[ci].Name != "Raph & Mikey, Troublemakers" {
				continue
			}
			for ai := range tbl.Cards[ci].Abilities {
				for pi := range tbl.Cards[ci].Abilities[ai].Prog {
					in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
					if in.Op == kop.OpDigUntil && in.C>>3&kop.DuFoundAttacking != 0 {
						in.C &^= kop.DuFoundAttacking << 3
						cleared++
					}
				}
			}
		}
		if cleared == 0 {
			t.Fatal("mutation found no Attacking DigUntil in Raph & Mikey")
		}
	}
	played, branch, stalls = map[string]int{}, map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kds-a", "kds-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		kdsScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			kczDump(t, gs)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestDigshapesLockstep(t *testing.T) {
	t.Parallel()

	games, divs, played, branch, stalls := runDigshapes(t, 256, false)
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	t.Logf("%d games, %d divergences; resolutions %s; kernel stalls %v; plays %v", games, divs, strings.Join(bs, " "), stalls, played)
	for stall, c := range stalls {
		if !strings.HasSuffix(stall, "EnterDefender") {
			t.Errorf("kernel stall %s in %d games", stall, c)
		}
	}
	for _, want := range []string{"echo", "raph"} {
		if branch[want] < kdsMin {
			t.Errorf("%s resolutions %d, want at least %d", want, branch[want], kdsMin)
		}
	}
}

// TestDigshapesMutation clears DuFoundAttacking from Raph & Mikey's DigUntil:
// the kernel's found creature then enters tapped but not attacking, so the
// lockstep must diverge.
func TestDigshapesMutation(t *testing.T) {
	t.Parallel()

	games, divs, _, _, _ := runDigshapes(t, 128, true)
	if divs == 0 {
		t.Fatalf("DuFoundAttacking mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
