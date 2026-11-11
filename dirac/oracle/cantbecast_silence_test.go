package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Silence and Ranger-Captain of Eos: a no-target Effect whose end-of-turn
// emblem static is "your opponents can't cast spells" (Silence) or "can't
// cast noncreature spells" (the Captain's sacrifice ability), played decision
// by decision against gorge. The opposing seat holds Lightning Bolts, and for
// the Captain also flash Ambush Vipers, so the emblem withholds the Bolt
// offers and (the Captain) leaves the Viper's. TestReproSilenceRangerCaptainReach
// replays the recorded games and requires each to reach the card; they are
// listed in TestStdSetReprosLockstep.
//
// The games are recorded by TestReproSilenceRangerCaptainRecord
// (CANT_BE_CAST_RECORD=1) on the kernel table built from the decks.
func silenceCases() []effectLongCase {
	bolts := append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...)
	boltsVipers := append(append(deckOf(6, "Lightning Bolt"), deckOf(4, "Ambush Viper")...), append(deckOf(8, "Mountain"), deckOf(6, "Forest")...)...)
	return []effectLongCase{
		{
			name:    "silence_opponents_cannot_cast_spells",
			prefer:  []string{"Silence"},
			decks:   [2][]string{append(deckOf(8, "Silence"), deckOf(16, "Plains")...), bolts},
			surface: "autopay",
			reached: movedFrom("Silence", state.ZStack, state.ZGraveyard),
		},
		{
			name:    "ranger_captain_sacrifice_opponents_cannot_cast_noncreature_spells",
			prefer:  []string{"Ranger-Captain of Eos"},
			decks:   [2][]string{append(deckOf(8, "Ranger-Captain of Eos"), deckOf(16, "Plains")...), boltsVipers},
			surface: "autopay",
			reached: movedFrom("Ranger-Captain of Eos", state.ZBattlefield, state.ZGraveyard),
		},
	}
}

// silencePolicy is cantBeCastPolicy, except that the Captain's sacrifice
// ability is activated whenever it is offered (so it is sacrificed while the
// opposing seat still holds a Bolt, the cast the emblem withholds).
func silencePolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := cantBeCastPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && boltHeld(p.G) {
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Ranger-Captain of Eos" {
					return o
				}
			}
		}
		return inner.Pick(p)
	})
}

// TestReproSilenceRangerCaptainReach replays the recorded games: no
// divergence, every scripted action offered, the card reached.
func TestReproSilenceRangerCaptainReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range silenceCases() {
		t.Run(c.name, func(t *testing.T) {
			silenceReplay(t, reg, c)
		})
	}
}

func silenceReplay(t *testing.T, reg *cards.Registry, c effectLongCase) {
	t.Helper()
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
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

// TestReproSilenceRangerCaptainRecord plays the cases' policies over seeds
// until one reaches the card, and writes the repro. It runs only with
// CANT_BE_CAST_RECORD=1.
func TestReproSilenceRangerCaptainRecord(t *testing.T) {
	if os.Getenv("CANT_BE_CAST_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range silenceCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, silencePolicy(m, c, seed))
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
