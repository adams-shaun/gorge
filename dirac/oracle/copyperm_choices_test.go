package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CopyPermanent Choices$ played decision by decision against gorge: the
// controller's mandatory pick (gorge's copypermanent_choice ask, whose options
// are "permanent") and the copy of it (kop.ExtCopyPicked). Calamity,
// Galloping Inferno's "choose a nonlegendary creature that saddled it this
// turn and create a tapped and attacking token that's a copy of it. Sacrifice
// that token at the beginning of the next end step. Repeat this process
// once." adds the tapped, attacking and end-step sacrifice riders, and gorge
// registers the delayed sacrifice even when no creature can be chosen.
// TestStdSetReprosLockstep replays the repros; TestCopypermChoicesReprosReach
// requires each recorded game to reach the shape it names, since a lockstep
// entry alone would pass a script that never made the copy.
//
// The games are recorded by TestCopypermChoicesRecord
// (COPYPERM_CHOICES_RECORD=1) on the case's pool; a card no Standard pool
// holds plays on the table its own decks need ("custom").
const calamity = "Calamity, Galloping Inferno"

type choiceCase struct {
	effectLongCase
	pool string
	// casts are cast in this order before anything else
	casts []string
	// attacker, when set, attacks whenever it can; saddle makes the policy
	// saddle it whenever the activation is offered
	attacker string
	saddle   bool
}

// calamityDecks is a 24-card deck of Calamity, the saddler and ten lands
// (the given extra basic standing in for some Mountains) against 24 Mountains.
func calamityDecks(saddler string) [2][]string {
	mine := append(deckOf(6, calamity), deckOf(8, saddler)...)
	mine = append(mine, deckOf(10, "Mountain")...)
	return [2][]string{mine, deckOf(24, "Mountain")}
}

func choiceCases() []choiceCase {
	return []choiceCase{
		{
			effectLongCase: effectLongCase{
				name:    "calamity_copies_a_saddler_tapped_and_attacking",
				decks:   calamityDecks("Reckless Lackey"),
				surface: "autopay",
				reached: calamityCopiedAttacking,
			},
			pool: "std-otj", casts: []string{"Reckless Lackey", calamity}, attacker: calamity, saddle: true,
		},
		{
			effectLongCase: effectLongCase{
				name:    "calamity_no_nonlegendary_saddler_still_schedules_the_sacrifice",
				decks:   calamityDecks("Magda, the Hoardmaster"),
				surface: "autopay",
				reached: calamityNothingToCopy,
			},
			pool: "std-otj", casts: []string{"Magda, the Hoardmaster", calamity}, attacker: calamity, saddle: true,
		},
		{
			effectLongCase: effectLongCase{
				name: "calix_copies_a_nonlegendary_enchantment",
				decks: [2][]string{
					append(append(append(append(deckOf(6, "Calix, Guided by Fate"), deckOf(3, "Spidersilk Armor")...), deckOf(3, "Fungal Bloom")...), deckOf(2, "Gerrard's Battle Cry")...),
						append(deckOf(6, "Forest"), deckOf(4, "Plains")...)...),
					deckOf(24, "Forest"),
				},
				surface: "autopay",
				reached: copiedAnEnchantment,
			},
			pool: "custom", casts: []string{"Calix, Guided by Fate", "Spidersilk Armor", "Fungal Bloom", "Gerrard's Battle Cry"}, attacker: "Calix, Guided by Fate",
		},
		{
			effectLongCase: effectLongCase{
				name: "good_king_mog_copies_a_moogle_token",
				decks: [2][]string{
					append(append(deckOf(6, "Summon: Good King Mog XII"), deckOf(8, "Ethereal Guidance")...), deckOf(10, "Plains")...),
					deckOf(24, "Plains"),
				},
				surface: "autopay",
				reached: copiedAToken,
			},
			pool: "custom", casts: []string{"Summon: Good King Mog XII", "Ethereal Guidance"},
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

// copiedAnEnchantment: a token copy of a nonlegendary enchantment was made.
func copiedAnEnchantment(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind != events.CopyToken || e.Obj == 0 {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name != "Calix, Guided by Fate" {
			return true
		}
	}
	return false
}

// copiedAToken: a token copy of a token was made.
func copiedAToken(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind != events.CopyToken || e.Obj == 0 {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.IsToken {
			return true
		}
	}
	return false
}

// choicePolicy casts the case's cards in order, saddles and attacks with its
// attacker, plays lands, and otherwise picks at random from the seed's own
// stream.
func choicePolicy(m *Matchup, c choiceCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	named := func(p *PickCtx, o Canon, name string) bool {
		if p.G == nil {
			return false
		}
		op, ok := p.G.GorgeOption(o)
		if !ok || op.Obj == 0 {
			return false
		}
		ob := p.G.E.G.Obj(op.Obj)
		return ob != nil && ob.Face() != nil && ob.Face().Name == name
	}
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DPriority:
			if c.saddle {
				for _, o := range p.D.Options {
					if o.Kind() == ActActivate && named(p, o, c.attacker) {
						return o
					}
				}
			}
			for _, want := range c.casts {
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
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack && named(p, o, c.attacker) {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestCopypermChoicesReprosReach replays the recorded games: no divergence,
// every scripted action offered, the shape each names reached.
func TestCopypermChoicesReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range choiceCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := reproSet(t, c.pool, r)
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

// TestCopypermChoicesRecord plays each case's policy over seeds until one
// reaches its shape, and writes the repro. It runs only with
// COPYPERM_CHOICES_RECORD=1; the games it writes are the committed repros.
func TestCopypermChoicesRecord(t *testing.T) {
	if os.Getenv("COPYPERM_CHOICES_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range choiceCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth " + c.pool}
				set := reproSet(t, c.pool, r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, choicePolicy(m, c, seed))
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
