package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// czaltdest pool. From Father to Son (a library search for a Vehicle) and
// Zuko's Conviction (a targeted graveyard return) both carry Forge's
// DestinationAlternative$/DestAltSVar$ MANDATORY X:
//
//   - From Father to Son: if this spell was cast from a graveyard
//     (Count$wasCastFromGraveyard), the searched Vehicle goes to the
//     battlefield instead of the hand;
//   - Zuko's Conviction: if kicked (Count$Kicked), the target creature card
//     is put onto the battlefield tapped instead of returned to its owner's
//     hand.
//
// gorge fixes the destination once, at the start of the resolution
// (changeZoneAltDestination, engine/effects/zone_change.go:60-117, from
// changeZonePrelude :168, before every path at :353). kgen lowers the two
// destinations as a branch, and this lockstep checks the arms against gorge
// across both card shapes. The pool's self-mill and discard fill the
// graveyards, so a flashback cast and a graveyard target are reachable.

var kczaltdestDecks = [2][]cardset.Entry{
	{
		{Name: "From Father to Son", Count: 4}, {Name: "Zuko's Conviction", Count: 4},
		{Name: "Carrion Cruiser", Count: 4}, {Name: "Veloheart Bike", Count: 2},
		{Name: "Voyager Glidecar", Count: 2}, {Name: "Daggerfang Duo", Count: 3},
		{Name: "Deathcap Marionette", Count: 3}, {Name: "Mineshaft Spider", Count: 3},
		{Name: "Pothole Mole", Count: 3}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "From Father to Son", Count: 4}, {Name: "Zuko's Conviction", Count: 4},
		{Name: "Carrion Cruiser", Count: 4}, {Name: "Midnight Mangler", Count: 2},
		{Name: "Cultivator's Caravan", Count: 2}, {Name: "Air Response Unit", Count: 2},
		{Name: "Daggerfang Duo", Count: 3}, {Name: "Deathcap Marionette", Count: 3},
		{Name: "Mineshaft Spider", Count: 3}, {Name: "Pothole Mole", Count: 2},
		{Name: "Under the Skin", Count: 2}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 5},
	},
}

func kczaltdestSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("czaltdest")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kczaltdestTally counts what the two carriers did, read from gorge's event
// log (the side that leads):
//
//   - "zuko_hand": a Zuko's Conviction resolution moved its target to the hand;
//   - "zuko_battlefield": the target went to the battlefield (kicked);
//   - "zuko_tapped": that battlefield entry tapped it;
//   - "fromfather_hand": a hand-cast From Father to Son (PutOnStack from hand);
//   - "fromfather_flashback": a flashback cast (PutOnStack from the graveyard);
//   - "fromfather_battlefield": its library search put the Vehicle onto the
//     battlefield.
type kczaltdestTally struct {
	n map[string]int
}

func (k *kczaltdestTally) add(key string) {
	if k.n == nil {
		k.n = map[string]int{}
	}
	k.n[key]++
}

// kczaltdestScan reads one game's gorge event log. A resolved spell is
// tracked until its own stack -> graveyard/exile MoveZone clears it, so a
// later unrelated move cannot be attributed to it.
func kczaltdestScan(g *GorgeSide, k *kczaltdestTally) {
	if g == nil {
		return
	}
	resolving := ""
	var resolved state.ObjID
	pendingTap := state.ObjID(0)
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ev.Text != "From Father to Son" {
				continue
			}
			if ev.From == state.ZGraveyard {
				k.add("fromfather_flashback")
			} else {
				k.add("fromfather_hand")
			}
		case events.Resolve:
			switch {
			case ev.Text == "Zuko's Conviction", ev.Text == "From Father to Son":
				resolving, resolved, pendingTap = ev.Text, ev.Obj, 0
			}
		case events.MoveZone:
			if ev.Obj == resolved && ev.From == state.ZStack {
				resolving, pendingTap = "", 0
				continue
			}
			switch resolving {
			case "Zuko's Conviction":
				if ev.From != state.ZGraveyard {
					continue
				}
				switch ev.To {
				case state.ZHand:
					k.add("zuko_hand")
				case state.ZBattlefield:
					k.add("zuko_battlefield")
					pendingTap = ev.Obj
				}
			case "From Father to Son":
				if ev.From == state.ZLibrary && ev.To == state.ZBattlefield {
					k.add("fromfather_battlefield")
				}
			}
		case events.Tap:
			if resolving == "Zuko's Conviction" && pendingTap != 0 && ev.Obj == pendingTap {
				k.add("zuko_tapped")
			}
		}
	}
}

// kczaltdestRun plays seeds 1..seeds of the matchup with the random policy.
// tbl is the (possibly mutated) table to play; stopOnDiv logs the first
// divergence.
func kczaltdestRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64, k *kczaltdestTally, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, kczaltdestDecks)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kczaltdest-a", "kczaltdest-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		kernelSide, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		rnd := RandomPolicy(seed)
		policy := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return rnd.Pick(c)
		})
		res := Lockstep(m, seed, kernelSide, policy, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(kernelSide)
		games++
		kczaltdestScan(gs, k)
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if stopOnDiv {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div = true
			if !stopOnDiv {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestCzaltdestLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kczaltdestSetup(t)
	tbl, _, _, err := set.Matchup(kczaltdestDecks[0], kczaltdestDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var tally kczaltdestTally
	games, div, stalls := kczaltdestRun(t, reg, set, tbl, 256, &tally, true)
	n := tally.n
	t.Logf("%d games, diverged %v, stalls %d, zuko hand %d battlefield %d tapped %d, from father hand %d flashback %d battlefield %d",
		games, div, stalls, n["zuko_hand"], n["zuko_battlefield"], n["zuko_tapped"],
		n["fromfather_hand"], n["fromfather_flashback"], n["fromfather_battlefield"])
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	for _, b := range []struct {
		name      string
		got, want int
	}{
		{"Zuko's Conviction to the hand", n["zuko_hand"], 20},
		{"Zuko's Conviction kicked to the battlefield", n["zuko_battlefield"], 10},
		{"From Father to Son hand-cast searches", n["fromfather_hand"], 20},
		{"From Father to Son flashback searches onto the battlefield", n["fromfather_battlefield"], 3},
	} {
		if b.got < b.want {
			t.Errorf("%s reached %d, want at least %d", b.name, b.got, b.want)
		}
	}
	// Tapped$ True applies only to the battlefield arm; a hand return never
	// taps. Every kicked battlefield entry taps the same object once.
	if n["zuko_tapped"] != n["zuko_battlefield"] {
		t.Errorf("Zuko's Conviction battlefield entries %d, tapped %d; want equal", n["zuko_battlefield"], n["zuko_tapped"])
	}
}

// TestCzaltdestMutation flips the first SkipIf's condition in both carriers'
// generated programs to never-true (the kernel then always takes the primary
// destination) before the run: gorge still takes the alternate destination
// when its condition holds, so the lockstep must diverge within 128 seeds.
func TestCzaltdestMutation(t *testing.T) {
	t.Parallel()

	reg, set := kczaltdestSetup(t)
	tbl, _, _, err := set.Matchup(kczaltdestDecks[0], kczaltdestDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	mut := kczaltdestAltNever(t, tbl)
	var tally kczaltdestTally
	if _, div, _ := kczaltdestRun(t, reg, set, mut, 128, &tally, false); !div {
		t.Fatalf("alternate destination never taken: no divergence in 128 seeds")
	}
}

// kczaltdestAltNever deep-copies a generated table and points the first
// SkipUnless of each carrier's program (the lowered alternate-destination
// SkipIf) at an always-false condition.
func kczaltdestAltNever(t *testing.T, tbl *kop.Table) *kop.Table {
	t.Helper()
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cp.Cards {
		abs := make([]kop.Ability, len(cp.Cards[i].Abilities))
		copy(abs, cp.Cards[i].Abilities)
		for j := range abs {
			if abs[j].Prog != nil {
				abs[j].Prog = append([]kop.Instr(nil), abs[j].Prog...)
			}
		}
		cp.Cards[i].Abilities = abs
	}
	cp.Filters = append([]kop.Filter(nil), tbl.Filters...)
	cp.Conds = append([]kop.Cond(nil), tbl.Conds...)
	// A fresh never-true condition (CondValue 0 >= 1) appended after the
	// table's own conditions; the carriers' first SkipUnless points at it.
	falseID := int16(len(cp.Conds))
	cp.Conds = append(cp.Conds, kop.Cond{Kind: kop.CondValue, A: kop.Const(0), Op: kop.CmpGE, B: kop.Const(1)})
	for _, name := range []string{"From Father to Son", "Zuko's Conviction"} {
		flipped := false
		for i := range cp.Cards {
			if cp.Cards[i].Name != name {
				continue
			}
			for a := range cp.Cards[i].Abilities {
				for j := range cp.Cards[i].Abilities[a].Prog {
					in := &cp.Cards[i].Abilities[a].Prog[j]
					if in.Op != kop.OpSkipUnless {
						continue
					}
					in.C = falseID
					flipped = true
					break
				}
				if flipped {
					break
				}
			}
			if flipped {
				break
			}
		}
		if !flipped {
			t.Fatalf("%s: no alternate-destination SkipIf in the table", name)
		}
	}
	return &cp
}
