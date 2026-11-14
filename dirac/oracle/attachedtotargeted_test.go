package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAttachedToTargetedLowers plays Fumble ("Return target creature to its
// owner's hand. Gain control of all Auras and Equipment that were attached to
// it, then attach them to another creature.") in lockstep with gorge over the
// attachedtotargeted fixture pool. Its chain reads `AttachedTo
// Targeted.Aura,Equipment` after the bounce, from last-known information, in
// both the GainControl and the Attach (Choices$ Creature) halves; kgen lowers
// the pair (dirac/kgen/lower_attachgroup.go) onto the remembered group
// (ExtRememberAll over F2AttachedToT0, which reads the attachments the host's
// departure left lingering), a multi-object GainControl and the
// destination-side attach pick (chooseAttachDest).
//
// The policy casts Fumble, Auras and Equipment, and equips, the moment one is
// offered, so a creature carrying an Aura and an Equipment is bounced in some
// games. Precondition: gorge's log shows Fumble resolutions that attached one
// and several Auras at once. The mutation subtest drops the kernel's attach and must
// diverge.
func TestAttachedToTargetedLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, tally := runAttachedToTargeted(t, 96, true, false)
		t.Logf("%d games, %d divergences, %d stalls; %+v", run.games, run.divs, run.stalls, tally)
		if tally.fumbles < 10 {
			t.Errorf("%d Fumble resolutions, want at least 10", tally.fumbles)
		}
		if tally.moved < 6 {
			t.Errorf("%d Fumbles moved at least one attachment, want at least 6", tally.moved)
		}
		if tally.multi < 2 {
			t.Errorf("%d Fumbles moved two or more attachments together, want at least 2", tally.multi)
		}
		if tally.stolen < 2 {
			t.Errorf("%d attachments changed controller, want at least 2", tally.stolen)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		run, _ := runAttachedToTargeted(t, 96, false, true)
		run.requireDivergence(t, "Fumble's attach dropped")
	})
}

var attachedToTargetedDecks = fixtureDecks{
	{{Name: "Fumble", Count: 6}, {Name: "Suntail Hawk", Count: 10}, {Name: "Pacifism", Count: 4}, {Name: "Holy Strength", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 7}},
	{{Name: "Fumble", Count: 6}, {Name: "Suntail Hawk", Count: 10}, {Name: "Pacifism", Count: 4}, {Name: "Holy Strength", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 7}},
}

var attachedToTargetedWatch = map[string]bool{"Fumble": true, "Pacifism": true, "Holy Strength": true}

// attachedToTargetedTally is what gorge's logs show of the Fumbles.
type attachedToTargetedTally struct {
	fumbles int // Fumble resolutions
	moved   int // ... that attached at least one object
	multi   int // ... that attached two or more objects
	stolen  int // attachments whose controller changed under a Fumble
}

// attachedToTargetedPolicy plays randomly but takes a cast or an activation
// of a watched card the moment one is offered (seven times in eight).
func attachedToTargetedPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x66756d62)) // "fumb"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k != ActActivate && k != ActCast {
					continue
				}
				if attachedToTargetedWatch[refCardName(c.G, a.Src())] && prng.IntN(8) != 0 {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runAttachedToTargeted(t *testing.T, seeds int, report, mutate bool) (fixtureRun, attachedToTargetedTally) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("attachedtotargeted")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range attachedToTargetedDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(attachedToTargetedDecks[0], attachedToTargetedDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	// precondition: Fumble is supported and its chain holds the lowered group
	var ops []kop.Op
	for _, cd := range tbl.Cards {
		if cd.Name == "Fumble" {
			for _, a := range cd.Abilities {
				for _, in := range a.Prog {
					ops = append(ops, in.Op)
				}
			}
		}
	}
	if !hasOp(ops, kop.OpGainControl) || !hasOp(ops, kop.OpAttachObj) || !hasOp(ops, kop.OpChooseCard) {
		t.Fatalf("Fumble's table program %v lacks the lowered GainControl / ChooseCard / AttachObj chain", ops)
	}
	if mutate {
		mutateCard(t, tbl, "Fumble", func(_ *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpAttachObj {
				in.Op = kop.OpNop
			}
		})
	}
	run := fixtureRun{casts: map[string]int{}}
	var tally attachedToTargetedTally
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"attachedtotargeted-a", "attachedtotargeted-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, attachedToTargetedPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		attachedToTargetedCount(g, &tally)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, tally
}

func hasOp(ops []kop.Op, op kop.Op) bool {
	for _, o := range ops {
		if o == op {
			return true
		}
	}
	return false
}

// attachedToTargetedCount adds g's Fumble resolutions to the tally: the
// Attach events between a Fumble's Resolve and the next Resolve are its
// attaches (the group is attached in the one sub-ability).
func attachedToTargetedCount(g *GorgeSide, tally *attachedToTargetedTally) {
	evs := g.E.L.Events
	for i := range evs {
		if evs[i].Kind != events.Resolve || refName(g, evs[i].Obj) != "Fumble" {
			continue
		}
		tally.fumbles++
		n := 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Resolve; j++ {
			switch evs[j].Kind {
			case events.Attach:
				if evs[j].Obj == 0 {
					continue
				}
				n++
			case events.ControlChange:
				tally.stolen++
			}
		}
		if n > 0 {
			tally.moved++
		}
		if n > 1 {
			tally.multi++
		}
	}
}

// refName is the first face's name of object id in g's game.
func refName(g *GorgeSide, id state.ObjID) string {
	o := g.E.G.Obj(id)
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return ""
	}
	return o.Card.Faces[0].Name
}
