package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// kassignNoCombatDamageDeck is the ticket's three fully-lowerable
// AssignNoCombatDamage carriers and the mana to cast them: Ophidian (U),
// Carrion Rats (B) and Dwarven Vigilantes (R), all with
// `DB$ Effect | StaticAbilities$ SNoCombatDamage | Duration$
// UntilHostLeavesPlayOrEOT`. Opt cycles to find them.
var kassignNoCombatDamageDeck = [2][]cardset.Entry{
	{
		{Name: "Ophidian", Count: 4},
		{Name: "Carrion Rats", Count: 4},
		{Name: "Dwarven Vigilantes", Count: 4},
		{Name: "Opt", Count: 4},
		{Name: "Island", Count: 8},
		{Name: "Swamp", Count: 8},
		{Name: "Mountain", Count: 8},
	},
	{
		{Name: "Ophidian", Count: 4},
		{Name: "Carrion Rats", Count: 4},
		{Name: "Dwarven Vigilantes", Count: 4},
		{Name: "Opt", Count: 4},
		{Name: "Island", Count: 8},
		{Name: "Swamp", Count: 8},
		{Name: "Mountain", Count: 8},
	},
}

// TestAssignNoCombatDamageLockstep plays the three carriers in lockstep and
// requires the Effect-delivered statics to reach gorge's continuous registry
// with the ticket's exact lifetime and host filter. Both sides then assign no
// combat damage: gorge's combatDamageAmount zeros the carrier and dirac's
// combatAmount reads kop.StatNoCombatDamage, so any damage the kernel dealt
// instead would surface as a projection divergence at the next decision.
func TestAssignNoCombatDamageLockstep(t *testing.T) {
	t.Parallel()
	// Resolve the corpus without git, like TestEffectStaticDurationLockstep:
	// pr-precheck's fresh-clone step runs a git archive with no .git.
	corpusDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no card corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range kassignNoCombatDamageDeck[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(kassignNoCombatDamageDeck[0], kassignNoCombatDamageDeck[1])
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 64
	var casts, registered int
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"assignnocomcombatdamage-a", "assignnocomcombatdamage-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		// Finish sees the gorge side's registry after the game. The registry
		// keeps every registration, live or expired, so a carrier's Effect
		// that resolved in any turn is visible here even if cleanup later
		// ended it.
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
			Finish: func(g *GorgeSide) {
				cont := g.E.RegisteredContinuous()
				for i := range cont {
					ce := &cont[i]
					if ce.Restriction != effects.ModeAssignNoCombatDamage {
						continue
					}
					// The precondition the zero-damage read depends on: the
					// registration is the Effect's host-bound static with the
					// ticket's duration, not a different AssignNoCombatDamage
					// shape.
					if !ce.UntilEOT || ce.Duration != "UntilHostLeavesPlayOrEOT" ||
						ce.RestrictParams["ValidCard"] != "Card.EffectSource" {
						t.Errorf("seed %d: AssignNoCombatDamage registration %+v lacks the host-bound EOT shape", seed, *ce)
					}
					registered++
				}
			}})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		for _, a := range res.Actions {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				casts++
			}
		}
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	if casts == 0 {
		t.Fatal("no cast or activation in the games; the lockstep proves nothing")
	}
	if registered == 0 {
		t.Fatal("no AssignNoCombatDamage Effect registration in any game; the carriers never resolved the static")
	}
	t.Logf("%d seeds, %d casts/activations, %d AssignNoCombatDamage registrations", seeds, casts, registered)
}
