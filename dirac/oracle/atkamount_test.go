package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Poetic Ingenuity ("whenever one or more
// Dinosaurs you control attack, create that many Treasure tokens") and The
// Earth King ("whenever one or more creatures you control with power 4 or
// greater attack, search your library for up to that many basic land
// cards"), both reading TriggerObjectsAttackers$Amount. The amount is the
// attackers that passed the line's own ValidAttackers$ filter, captured once
// per declaration: the kernel's fireAttacksGen binds attackersMatching over
// the whole declaration as the trigger amount. The decks attack with the
// carriers' matching creatures beside smaller non-matching ones so the
// matched count is regularly smaller than the declared total, and with two
// or more matching attackers so the amount is not just one.
//
// The test observes gorge's own TriggerPush events (the batch line fires
// once per declaration) and re-derives the two counts from the attacking
// board at that point. A wrong amount moves a different number of Treasures
// or basic lands, which the very next projection comparison sees.

var katkamountDecks = [2][]cardset.Entry{
	{
		{Name: "Poetic Ingenuity", Count: 2}, {Name: "The Earth King", Count: 2},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Frenzied Raptor", Count: 2},
		{Name: "Raptor Hatchling", Count: 2}, {Name: "Craw Wurm", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Lightning Bolt", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Poetic Ingenuity", Count: 2}, {Name: "The Earth King", Count: 2},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Frenzied Raptor", Count: 2},
		{Name: "Raptor Hatchling", Count: 2}, {Name: "Craw Wurm", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Shock", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 9},
	},
}

// katkamountPolicy plays the random policy but, whenever an attack is
// offered, picks only among the attack options: every able creature joins
// the declaration, so the non-matching small creatures really do attack
// beside the carriers' matching ones. Otherwise it prefers lands, casts and
// activations over passing, and falls back to the random option.
func katkamountPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		hasAtk := false
		for _, a := range c.D.Options {
			if a.Kind() == ActAttack {
				hasAtk = true
				buf = append(buf, a)
			}
		}
		if !hasAtk {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActPlayLand || k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// katkamountTally counts the carrier trigger events and the two board
// shapes the test asks for. evAt is the event-log cursor, reset when a
// fresh game's log is shorter (a new GorgeSide per game).
type katkamountTally struct {
	evAt     int
	triggers int // carrier AttackersDeclared trigger pushes observed
	smaller  int // matched count < declared attackers
	multi    int // matched count >= 2
	poetic   int
	king     int
}

func hasType(f *cards.Face, want string) bool {
	for _, t := range f.Types {
		if t == want {
			return true
		}
	}
	return false
}

// check scans the new gorge events for a carrier's AttackersDeclared
// TriggerPush and re-derives the declaration shape from the attacking
// board. It never reports a divergence itself.
func (c *katkamountTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		c.evAt = j + 1
		if e.Kind != events.TriggerPush {
			continue
		}
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		var dino bool
		switch o.Face().Name {
		case "Poetic Ingenuity":
			dino = true
		case "The Earth King":
			dino = false
		default:
			continue
		}
		if int(e.Amount) >= len(o.Face().Triggers) || o.Face().Triggers[e.Amount].Mode != "AttackersDeclared" {
			continue
		}
		declared, matched := 0, 0
		for _, id := range g.Zone(state.ZBattlefield, e.Player) {
			ob := g.Obj(id)
			if ob == nil || !ob.IsAttacking || ob.Face() == nil {
				continue
			}
			declared++
			f := ob.Face()
			if dino {
				if hasType(f, "Dinosaur") {
					matched++
				}
			} else if f.IsCreature() && f.Power() >= 4 {
				matched++
			}
		}
		c.triggers++
		if dino {
			c.poetic++
		} else {
			c.king++
		}
		if matched < declared {
			c.smaller++
		}
		if matched >= 2 {
			c.multi++
		}
	}
	return nil
}

func katkamountSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("atkamount")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// katkamountMutate rewrites every AttackersDeclared carrier's built filter
// to 0, "any attacker": the kernel then binds the whole declaration as the
// amount instead of the matched subset, so the effect count must diverge.
func katkamountMutate(tbl *kop.Table) int {
	n := 0
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			ab := &tbl.Cards[i].Abilities[j]
			if ab.Kind == kop.AbTriggered && ab.Trig.Event == kop.TrigAttackersDeclared && ab.Trig.Filter != 0 {
				ab.Trig.Filter = 0
				n++
			}
		}
	}
	return n
}

// katkamountRun plays seeds 1..seeds of the matchup; mutate rewrites the
// kernel table's AttackersDeclared filters first. stopOnDiv returns at the
// first divergence (the mutation test only needs existence).
func katkamountRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, mutate, stopOnDiv bool) (games int, div bool, stalls int, tally *katkamountTally) {
	t.Helper()
	decks := kcastDeck(t, reg, katkamountDecks)
	tbl, _, _, err := set.Matchup(katkamountDecks[0], katkamountDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		if n := katkamountMutate(tbl); n != 2 {
			t.Fatalf("mutated %d AttackersDeclared filters, want 2", n)
		}
	}
	tally = &katkamountTally{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"katkamount-a", "katkamount-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.evAt = 0
		res := Lockstep(m, seed, k, katkamountPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tally.check})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div = true
			if !mutate {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			} else {
				t.Logf("mutation seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
			}
			if stopOnDiv {
				return
			}
		}
	}
	return
}

func TestAtkamountLockstep(t *testing.T) {
	t.Parallel()

	reg, set := katkamountSetup(t)
	games, div, stalls, tally := katkamountRun(t, reg, set, 256, false, true)
	t.Logf("%d games, diverged %v, stalls %d, carrier triggers %d (Poetic %d, Earth King %d), matched<declared %d, matched>=2 %d",
		games, div, stalls, tally.triggers, tally.poetic, tally.king, tally.smaller, tally.multi)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if tally.smaller < 20 {
		t.Errorf("carrier triggers with matched < declared reached %d, want at least 20", tally.smaller)
	}
	if tally.multi < 10 {
		t.Errorf("carrier triggers with matched count 2 or more reached %d, want at least 10", tally.multi)
	}
}

// TestAtkamountMutation: binding the whole declaration instead of the
// matched subset (Trigger.Filter = 0) must make the lockstep diverge.
func TestAtkamountMutation(t *testing.T) {
	reg, set := katkamountSetup(t)
	_, div, _, tally := katkamountRun(t, reg, set, 64, true, true)
	if !div {
		t.Fatalf("filter=0 mutation: no divergence in 64 seeds (carrier triggers %d, matched<declared %d, matched>=2 %d)",
			tally.triggers, tally.smaller, tally.multi)
	}
}
