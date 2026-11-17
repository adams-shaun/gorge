package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
)

// CR 605.1a, 601.2g: a multi-mana or dynamic creature host (Llanowar Tribe's
// {G}{G}{G}, Priest of Titania's {G} per Elf) under a Defer-class AttachedBy
// Taps watcher (Cryoshatter) is a source gorge's planner defers: its census
// never lists it. The kernel's script route modelled that gap only for a
// plain one-mana source, so the host kept its ordinary census unit and the
// offer priced casts gorge's census does not. The recordings play on the pool
// their own decks need (pool custom).
func mbcDeferredMultiManaCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "deferred_multimana_host_funds_no_cast",
			prefer: []string{"Cryoshatter", "Llanowar Tribe"},
			decks: [2][]string{
				append(deckOf(8, "Cryoshatter"), deckOf(16, "Island")...),
				append(deckOf(4, "Colossal Dreadmaw", deckOf(4, "Llanowar Tribe")...), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return cryoshatterDorkPriority(g, "Llanowar Tribe", 1) },
		},
		{
			name:   "deferred_dynamic_host_funds_no_cast",
			prefer: []string{"Cryoshatter", "Priest of Titania"},
			decks: [2][]string{
				append(deckOf(8, "Cryoshatter"), deckOf(16, "Island")...),
				append(deckOf(4, "Colossal Dreadmaw", deckOf(4, "Priest of Titania", deckOf(2, "Llanowar Elves")...)...), deckOf(14, "Forest")...),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return cryoshatterDorkPriority(g, "Priest of Titania", 1) },
		},
	}
}

// mbcDeferredWhy is the reason each recording carries.
var mbcDeferredWhy = map[string]string{
	"deferred_multimana_host_funds_no_cast": "Cryoshatter on an untapped Llanowar Tribe: gorge's census never lists the deferred Tribe, so its {G}{G}{G} funds no Colossal Dreadmaw on the autopay surface",
	"deferred_dynamic_host_funds_no_cast":   "Cryoshatter on an untapped Priest of Titania: gorge's census never lists the deferred Priest, so its {G} per Elf funds no Colossal Dreadmaw on the autopay surface",
}

// TestMBCDeferredMultiManaHostLockstep replays the recorded games: no
// divergence, every scripted action offered, the host reached enchanted and
// untapped on its controller's priority.
func TestMBCDeferredMultiManaHostLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range mbcDeferredMultiManaCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches the host of %s", c.name)
			}
		})
	}
}

// TestMBCDeferredMultiManaHostRecord plays the cases' policies over seeds
// until one reaches the host's priority, and writes the repro. It runs only
// with PINS_RECORD=1.
func TestMBCDeferredMultiManaHostRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range mbcDeferredMultiManaCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom", Why: mbcDeferredWhy[c.name]}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if !mbcDeferredDiverges(t, reg, c, r) {
					continue // the game never prices a cast the host's ordinary unit would fund
				}
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached the host of %s", c.name)
		})
	}
}

// mbcDeferredDiverges replays recording r with the gap's several-mana shapes
// dropped (dirac.DebugNoTapsDeferYield): the host is then an ordinary census
// unit, as the kernel kept it before the gap modelled its production. It reports whether the game diverges from gorge, that is
// whether the recording prices a cast only the host's ordinary unit funds.
func mbcDeferredDiverges(t *testing.T, reg *cards.Registry, c effectLongCase, r *Repro) bool {
	t.Helper()
	dirac.DebugNoTapsDeferYield = true
	defer func() { dirac.DebugNoTapsDeferYield = false }()
	res, _ := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), &ScriptPolicy{Actions: r.Actions})
	return res.Div != nil
}

// TestMBCDeferredMultiManaHostMutation: with the several-mana gap dropped each
// recorded game diverges from gorge, proof the recordings exercise the
// deferral of the multi-mana host (the precondition the replay's pass rests on).
func TestMBCDeferredMultiManaHostMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	for _, c := range mbcDeferredMultiManaCases() {
		r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !mbcDeferredDiverges(t, reg, c, r) {
			t.Errorf("%s: no divergence with the several-mana gap dropped: the recording does not exercise the deferral", c.name)
		}
	}
}
