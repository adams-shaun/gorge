package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnattachedEquipmentLowers: the Mode$ Unattached Equipment family
// (gorge trigmatch unattachedMatches, CR 701.3b/d) in lockstep over the
// unattachequip fixture pool. Captain's Hook, Grafted Wargear and Stitcher's
// Graft print `T:Mode$ Unattached | ValidAttachment$ Card.Self | ValidObject$
// Permanent` and destroy or sacrifice that permanent; Grafted Exoskeleton
// rides the same trigger but stays refused for its Infect. The kernel fires
// the trigger at the boundary where gorge publishes the detach
// (dirac/trig_unattached.go):
//
//   - re-equip: gorge rules/emit.go publishes the Unattached of the FORMER
//     bearer before the Attach (a re-attach to a different creature);
//   - bearer death: the bearer leaves the battlefield, gorge's Move fold and
//     attachmentSBAs publish the detach at the SBA checkpoint, the bearer
//     already gone (the body is a no-op, the trigger still goes on the
//     stack);
//   - bearer bounce: the same, with an Ephemerate returning the bearer under
//     the same ObjID before the publish -- the trigger object is the OLD
//     object, a new object now (CR 400.7), so the body is a no-op twice
//     over;
//   - the Equipment's own destruction while attached (gorge's CR 603.10a
//     look-back walk, looksBackOnDeparture): Disenchant and Shatter, and a
//     Karn's Touch animation that turns the carrier into a killable
//     creature;
//   - the CR 704.5p SBA arm: a carrier Karn's Touch animated is an attached
//     creature, it detaches and its trigger fires ("attached creature or
//     battle becomes unattached").
//
// The finish hook counts the publishes off gorge's finished event log and
// fails the run when a path the pool is built to reach was never reached; a
// refused carrier fails the pool build, so the test cannot pass vacuously.
var kunattDecks = [2][]cardset.Entry{
	{
		{Name: "Captain's Hook", Count: 2}, {Name: "Grafted Wargear", Count: 2}, {Name: "Stitcher's Graft", Count: 2},
		{Name: "Karn's Touch", Count: 2}, {Name: "Ephemerate", Count: 2},
		{Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 1},
		{Name: "Disenchant", Count: 2}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Plains", Count: 7}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Captain's Hook", Count: 2}, {Name: "Grafted Wargear", Count: 2}, {Name: "Stitcher's Graft", Count: 2},
		{Name: "Karn's Touch", Count: 2}, {Name: "Ephemerate", Count: 1},
		{Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2}, {Name: "Hill Giant", Count: 1},
		{Name: "Disenchant", Count: 2}, {Name: "Shatter", Count: 2}, {Name: "Doom Blade", Count: 2}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

const kunattReattachText = "reattached to a new permanent"
const kunattBearerLeftText = "bearer left the battlefield"
const kunattSBAAnimText = "attached creature or battle becomes unattached"
const kunattSBANonAttachText = "non-attachment permanent becomes unattached"

// kunattCounts is read off gorge's finished event log.
type kunattCounts struct {
	publishes    int // events.Unattached, all arms
	reattach     int // ... the re-equip arm
	bearerLeft   int // ... the bearer-left arm, the bearer never returning
	bounce       int // ... the bearer-left arm, the bearer returning (a flicker)
	leftAttached int // a carrier left the battlefield while attached (the CR 603.10a look-back arm; no Unattached event)
	sbaAnim      int // ... the CR 704.5p arms, a carrier an animator made a creature (either attachedKindIllegal text)

	killed int // a bearer alive at its publish left the battlefield later (the trigger's Destroy/SacrificeAll, or later removal)
}

func kunattFinish(c *kunattCounts) func(*GorgeSide) {
	return func(g *GorgeSide) {
		bearer := map[state.ObjID]state.ObjID{} // attachment -> the bearer it is attached to
		entered := map[state.ObjID]uint64{}     // object -> the seq of its last battlefield entry
		leftAt := map[state.ObjID]uint64{}      // object -> the seq it last left the battlefield
		live := map[state.ObjID]bool{}          // objects known on the battlefield
		killedAt := map[state.ObjID]uint64{}    // bearers on the battlefield at their detach
		type pending struct {
			b   state.ObjID
			seq uint64
		}
		var pend []pending // bearer-left publishes awaiting a possible return
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			switch e.Kind {
			case events.MoveZone:
				if e.To == state.ZBattlefield {
					live[e.Obj] = true
					entered[e.Obj] = e.Seq
					delete(leftAt, e.Obj)
				}
				if e.From == state.ZBattlefield && e.To != state.ZBattlefield {
					live[e.Obj] = false
					leftAt[e.Obj] = e.Seq
					if bearer[e.Obj] != 0 {
						c.leftAttached++
						bearer[e.Obj] = 0
					}
				}
			case events.Attach:
				if len(e.IDs) > 0 {
					bearer[e.Obj] = e.IDs[0]
				} else {
					bearer[e.Obj] = 0
				}
			case events.Unattached:
				c.publishes++
				b := state.ObjID(0)
				if len(e.IDs) > 0 {
					b = e.IDs[0]
				}
				switch e.Text {
				case kunattReattachText:
					c.reattach++
				case kunattBearerLeftText:
					if live[b] {
						// the bearer is back on the battlefield already: a
						// flicker's return between the detach and the SBA
						// checkpoint gorge publishes at (the trigger binds it
						// live, and the body still reaches it)
						c.bounce++
					} else {
						c.bearerLeft++
						pend = append(pend, pending{b, e.Seq})
					}
				case kunattSBAAnimText, kunattSBANonAttachText:
					c.sbaAnim++
				}
				if b != 0 && live[b] {
					killedAt[b] = e.Seq
				}
				bearer[e.Obj] = 0
			}
		}
		for _, p := range pend {
			if entered[p.b] > p.seq {
				c.bounce++
				c.bearerLeft--
			}
		}
		// a bearer alive at its publish that left the battlefield afterwards:
		// the trigger's Destroy/SacrificeAll (or some later removal) reached it
		for b, seq := range killedAt {
			if leftAt[b] > seq {
				c.killed++
			}
		}
	}
}

// kunattPolicy is castFirstPolicy with a cap on an object's activations per
// turn: a free Equip (Grafted Wargear, Equip {0}) can be re-activated
// endlessly at priority, and the kernel counts an object's non-mana
// activations in seven bits (StallActs at 127). Six per object per turn
// still re-equips freely; everything else plays as castFirst.
func kunattPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	seen := map[uint32]int{}
	turn := -1
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if t := c.G.Turn(); t != turn {
				turn, seen = t, map[uint32]int{}
			}
		}
		buf := make([]Canon, 0, len(c.D.Options))
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				if seen[uint32(a.Src())] >= 6 {
					continue
				}
				seen[uint32(a.Src())]++
			}
			buf = append(buf, a)
		}
		if len(buf) == len(c.D.Options) {
			return base.Pick(c)
		}
		if len(buf) == 0 {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestUnattachedEquipmentLowers(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unattachequip")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kunattDecks)
	tbl, _, _, err := set.Matchup(kunattDecks[0], kunattDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(96)
	if v, e := strconv.Atoi(os.Getenv("UNATT_EQUIP_SEEDS")); e == nil && v > 0 {
		seeds = uint64(v)
	}
	var c kunattCounts
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"kunatt-a", "kunatt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kunattPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Finish: kunattFinish(&c)})
		CloseSide(k)
		if s := res.Stall; len(s) >= len(KernelStallPrefix) && s[:len(KernelStallPrefix)] == KernelStallPrefix {
			t.Fatalf("seed %d: kernel stall %s", seed, s)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds: %d publishes (%d re-equip, %d bearer left, %d bounce, %d 704.5p), %d carriers left attached, %d bearers killed after a publish",
		seeds, c.publishes, c.reattach, c.bearerLeft, c.bounce, c.sbaAnim, c.leftAttached, c.killed)
	if c.reattach < 5 {
		t.Fatalf("only %d re-equip publishes over %d seeds: the pool does not exercise it", c.reattach, seeds)
	}
	if c.bearerLeft < 3 {
		t.Fatalf("only %d bearer-left publishes over %d seeds: the pool does not exercise it", c.bearerLeft, seeds)
	}
	if c.bounce < 1 {
		t.Fatalf("no bearer bounced (an Ephemerate return before the detach published) over %d seeds", seeds)
	}
	if c.leftAttached < 1 {
		t.Fatalf("no carrier left the battlefield while attached (the CR 603.10a look-back arm) over %d seeds", seeds)
	}
	// No minimum on sbaAnim: the CR 704.5p arm needs a Karn's Touch that
	// animates an ATTACHED carrier, and castFirst's first legal target is
	// always an unattached Equipment while one exists, so the pool cannot
	// promise one. The arm is tallied when it fires.
	if c.killed < 3 {
		t.Fatalf("only %d bearers killed after a publish over %d seeds: the Destroy/SacrificeAll body does not run", c.killed, seeds)
	}
	if c.publishes < 12 {
		t.Fatalf("only %d Unattached publishes over %d seeds: the pool does not exercise the family", c.publishes, seeds)
	}
	if c.killed < 1 {
		t.Fatalf("no bearer was removed after its publish (the trigger's Destroy/SacrificeAll body) over %d seeds", seeds)
	}
}
