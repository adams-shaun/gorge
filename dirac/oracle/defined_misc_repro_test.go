package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// The Cloning of Shredder's chapters II and III copy "a card exiled with this
// Saga" (CopyPermanent Defined$ ExiledWith, kop.ExtCopyExiledWith). Chapter I
// exiles a creature card from the graveyard and copies it from the resolution's
// Remembered set; the later chapters must find the same card through the
// exiled-with link. A game that reaches chapter II has made two token copies,
// so the shape's reach is two CopyToken events (chapter I's Remembered copy and
// chapter II's ExiledWith copy). TestStdSetReprosLockstep replays the repro;
// TestDefinedMiscReprosReach requires the recorded game to reach the shape.
//
// The game is recorded by TestDefinedMiscRecord (DEFINED_MISC_RECORD=1) on the
// std-tmt pool.
const shredderExiledCopyFn = "cloning_shredder_copies_the_card_exiled_with_it"

func shredderExiledCopyCase() effectLongCase {
	return effectLongCase{
		name:   shredderExiledCopyFn,
		prefer: []string{"Paramecia Coloniex", "The Cloning of Shredder"},
		decks: [2][]string{
			append(append(append(deckOf(4, "The Cloning of Shredder"), deckOf(4, "Paramecia Coloniex")...),
				deckOf(6, "Zoo Escapees")...), deckOf(10, "Swamp")...),
			deckOf(24, "Swamp"),
		},
		surface: "autopay",
		reached: shredderCopiedExiledCard,
	}
}

// shredderCopiedExiledCard: the Saga made a token copy from its Remembered set
// and then a second one from its exiled-with set.
func shredderCopiedExiledCard(g *GorgeSide) bool {
	copies := 0
	for _, e := range g.E.L.Events {
		if e.Kind == events.CopyToken {
			copies++
		}
	}
	return copies >= 2
}

// TestDefinedMiscReprosReach replays the recorded game: no divergence, every
// scripted action offered, and the exiled-with copy reached.
func TestDefinedMiscReprosReach(t *testing.T) {
	reg := synthReg(t)
	c := shredderExiledCopyCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := reproSet(t, "std-tmt", r)
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, set, script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never reached the exiled-with copy")
	}
}

// TestDefinedMiscRecord plays the case's policy over seeds until one reaches
// the shape, and writes the repro. It runs only with DEFINED_MISC_RECORD=1.
func TestDefinedMiscRecord(t *testing.T) {
	if os.Getenv("DEFINED_MISC_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := shredderExiledCopyCase()
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-tmt"}
		set := reproSet(t, "std-tmt", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
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
	t.Fatalf("no seed reached the exiled-with copy")
}
