package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Calamity, Galloping Inferno's "choose a nonlegendary creature that saddled
// it this turn and create a tapped and attacking token that's a copy of it.
// Sacrifice that token at the beginning of the next end step. Repeat this
// process once." (CopyPermanent Choices$ | TokenTapped$ | TokenAttacking$ |
// AtEOT$ Sacrifice, twice) played decision by decision against gorge: the
// controller's pick, the copy entering tapped and attacking, and the delayed
// sacrifice, which gorge registers even when no creature can be chosen
// (kop.ExtCopyPicked). TestStdSetReprosLockstep replays the repros;
// TestCalamityChoicesReprosReach requires each recorded game to reach the
// shape it names, since a lockstep entry alone would pass a script that never
// attacked with a saddled Calamity.
//
// The games are recorded by TestCalamityChoicesRecord (CALAMITY_CHOICES_RECORD=1)
// on the std-otj pool.
const calamity = "Calamity, Galloping Inferno"

type calamityCase struct {
	effectLongCase
	// saddler is the creature the policy saddles with first (cast before any other)
	saddler string
}

// calamityDecks is a 24-card deck of Calamity, the saddler and ten lands
// (the given extra basic standing in for some Mountains) against 24 Mountains.
func calamityDecks(saddler, extra string) [2][]string {
	mine := append(deckOf(6, calamity), deckOf(8, saddler)...)
	mine = append(mine, deckOf(5, "Mountain")...)
	mine = append(mine, deckOf(5, extra)...)
	return [2][]string{mine, deckOf(24, "Mountain")}
}

func calamityCases() []calamityCase {
	return []calamityCase{
		{
			effectLongCase: effectLongCase{
				name:    "calamity_copies_a_saddler_tapped_and_attacking",
				decks:   calamityDecks("Reckless Lackey", "Mountain"),
				surface: "autopay",
				reached: calamityCopiedAttacking,
			},
			saddler: "Reckless Lackey",
		},
		{
			effectLongCase: effectLongCase{
				name:    "calamity_no_nonlegendary_saddler_still_schedules_the_sacrifice",
				decks:   calamityDecks("Magda, the Hoardmaster", "Mountain"),
				surface: "autopay",
				reached: calamityNothingToCopy,
			},
			saddler: "Magda, the Hoardmaster",
		},
	}
}

// calamityCopiedAttacking: tapped and attacking copies were made of two
// different creatures, so a pick was made among several saddlers.
func calamityCopiedAttacking(g *GorgeSide) bool {
	srcs := map[state.ObjID]bool{}
	for _, e := range g.E.L.Events {
		if e.Kind == events.CopyToken && e.Amount&events.CopyTokenAttacking != 0 && e.Amount&events.CopyTokenTapped != 0 {
			srcs[e.Obj] = true
		}
	}
	return len(srcs) >= 2
}

// calamityNothingToCopy: Calamity's attack trigger registered its delayed
// sacrifice groups with no copy made (only legendary saddlers).
func calamityNothingToCopy(g *GorgeSide) bool {
	reg, copies := 0, 0
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.CopyToken:
			copies++
		case e.Kind == events.DelayedRegister && e.Counter == "__kwEncoreSacrificeGroup" && len(e.IDs) == 0:
			reg++
		}
	}
	return reg >= 2 && copies == 0
}

// calamityPolicy casts the saddler and Calamity before anything else, saddles
// Calamity whenever offered, attacks with it, and otherwise plays a land or
// picks at random from the seed's own stream.
func calamityPolicy(m *Matchup, c calamityCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DPriority:
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && p.G != nil {
					if op, ok := p.G.GorgeOption(o); ok && op.Obj != 0 {
						if ob := p.G.E.G.Obj(op.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == calamity {
							return o
						}
					}
				}
			}
			for _, want := range []string{c.saddler, calamity} {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		case DAttackers:
			if p.G != nil {
				for _, o := range p.D.Options {
					if op, ok := p.G.GorgeOption(o); ok && op.Obj != 0 {
						if ob := p.G.E.G.Obj(op.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == calamity {
							return o
						}
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestCalamityChoicesReprosReach replays the recorded games: no divergence,
// every scripted action offered, the shape each names reached.
func TestCalamityChoicesReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-otj")
	for _, c := range calamityCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never reaches the shape")
			}
		})
	}
}

// TestCalamityChoicesRecord plays each case's policy over seeds until one
// reaches its shape, and writes the repro. It runs only with
// CALAMITY_CHOICES_RECORD=1; the games it writes are the committed repros.
func TestCalamityChoicesRecord(t *testing.T) {
	if os.Getenv("CALAMITY_CHOICES_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-otj")
	for _, c := range calamityCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-otj"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, calamityPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
