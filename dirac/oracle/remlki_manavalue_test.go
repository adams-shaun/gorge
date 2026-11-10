package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Reanimate returns a creature card it remembers, then the controller loses
// life equal to its mana value (RememberedLKI$CardManaCost); Grave
// Researcher's prepared face is a copy of that spell. TestStdSetReprosLockstep
// replays the recorded games; the cases reuse the mass writers' recorder.
func remLKIManaValueCases() []rememberMassCase {
	return []rememberMassCase{
		{rememberChoiceCase: rememberChoiceCase{
			// the opponent's Disfigure kills a Walking Corpse; Reanimate returns
			// it and its controller loses two life
			charmRepeatCase: charmRepeatCase{
				name: "return_remembers_lki_mana_value_reanimate", card: "Reanimate", prefer: []string{"Walking Corpse", "Disfigure"},
				decks: [2][]string{
					append(deckOf(6, "Reanimate"), append(deckOf(6, "Walking Corpse"), deckOf(12, "Swamp")...)...),
					append(deckOf(8, "Disfigure"), deckOf(16, "Swamp")...),
				},
				kind: events.LifeChange, n: 1,
			},
			cardName: "Reanimate",
		}},
		{rememberChoiceCase: rememberChoiceCase{
			// three Zombies die to the opponent's Disfigure, so at the upkeep
			// the Researcher becomes prepared and its controller casts the
			// spell copy on one of them
			charmRepeatCase: charmRepeatCase{
				name: "prepared_copy_remembers_lki_mana_value_grave_researcher", card: "Grave Researcher", prefer: []string{"Reanimate", "Walking Corpse", "Disfigure"},
				decks: [2][]string{
					append(deckOf(3, "Grave Researcher"), append(deckOf(9, "Walking Corpse"), deckOf(12, "Swamp")...)...),
					append(deckOf(10, "Disfigure"), deckOf(14, "Swamp")...),
				},
				kind: events.LifeChange, n: 1,
			},
			cardName: "Reanimate",
		}},
	}
}

// remLKIPolicy is charmRepeatPolicy that also takes a prepared copy's cast
// when one is offered.
func remLKIPolicy(m *Matchup, c charmRepeatCase) Policy {
	base := charmRepeatPolicy(m, c)
	return PolicyFunc(func(p *PickCtx) Canon {
		for _, a := range p.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == faceCodePrepared {
				return a
			}
		}
		return base.Pick(p)
	})
}

// TestRemLKIManaValueReprosResolve replays the recorded games: no
// divergence, every scripted action offered, and the life loss made during
// the spell's resolution.
func TestRemLKIManaValueReprosResolve(t *testing.T) {
	reg := synthReg(t)
	for _, c := range remLKIManaValueCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hits := rememberMassPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if hits < c.n {
				t.Fatalf("%d %v events from the %s resolution, want at least %d", hits, c.kind, c.cardName, c.n)
			}
		})
	}
}

// TestRemLKIManaValueRecord plays the cases' scripted policies over seeds
// until one resolves the card with its effect, and writes the repro. It runs
// only with REMLKI_MV_RECORD=1.
func TestRemLKIManaValueRecord(t *testing.T) {
	if os.Getenv("REMLKI_MV_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range remLKIManaValueCases() {
		t.Run(c.name, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, hits := rememberMassPlay(t, reg, c, r, set, remLKIPolicy(m, c.charmRepeatCase))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if hits < c.n {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, %d events", seed, len(res.Actions), hits)
				return
			}
			t.Fatalf("no seed resolved %s with its effect", c.cardName)
		})
	}
}
