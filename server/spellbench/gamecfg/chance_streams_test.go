package gamecfg_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

// TestEnvChanceDrawsComeFromSecrets pins the env's chance planner: a coin flip
// is the shared chance stream's value, two games with the same secret and the
// same intents agree, and a different secret changes the flip. No draw is
// unplanned.
func TestEnvChanceDrawsComeFromSecrets(t *testing.T) {
	reg := testcorpus.Registry(t)
	coin := chanceCoinCard(t)
	decks := [2][]*cards.Card{make([]*cards.Card, 12), make([]*cards.Card, 12)}
	for i := range decks[0] {
		decks[0][i] = coin
		decks[1][i] = coin
	}
	sec := secret(41)
	a, err := gamecfg.New(reg, sec, decks, gamecfg.Rules{Mulligan: "none"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := gamecfg.New(reg, sec, decks, gamecfg.Rules{Mulligan: "none"})
	if err != nil {
		t.Fatal(err)
	}
	intents, flip := driveFirstCoinFlip(t, a)
	if want := sec.Stream("shared", "chance", 0).IntN(2); flip != want {
		t.Fatalf("flip = %d, want the shared chance stream's %d", flip, want)
	}
	for i, in := range intents {
		if err := b.Submit(in); err != nil {
			t.Fatalf("twin intent %d: %v", i, err)
		}
	}
	if a.E.L.Head() != b.E.L.Head() {
		t.Fatalf("same secret and intents reached different states: %s vs %s", a.E.L.Head(), b.E.L.Head())
	}
	if err := a.CheckRandomness(); err != nil {
		t.Fatalf("planned flip counted as unplanned: %v", err)
	}
	if err := b.CheckRandomness(); err != nil {
		t.Fatalf("twin planned flip counted as unplanned: %v", err)
	}

	changed := false
	for s := 0; s < 16 && !changed; s++ {
		other, err := gamecfg.New(reg, secret(byte(s)), decks, gamecfg.Rules{Mulligan: "none"})
		if err != nil {
			t.Fatal(err)
		}
		_, otherFlip := driveFirstCoinFlip(t, other)
		changed = otherFlip != flip
	}
	if !changed {
		t.Fatal("no different secret seed changed the flip")
	}
}

// driveFirstCoinFlip casts the coin spell at the first opportunity and
// returns the intents submitted and the flip's value.
func driveFirstCoinFlip(t *testing.T, g *gamecfg.Game) ([]decision.Intent, int) {
	t.Helper()
	var intents []decision.Intent
	cast := false
	before := len(g.E.ChanceTranscript())
	for n := 0; n < 200; n++ {
		d := g.E.Pending()
		if d == nil {
			t.Fatal("game ended before the coin flip")
		}
		choice := -1
		if !cast {
			for i, o := range d.Options {
				if o.Kind == "cast" {
					choice = i
					cast = true
					break
				}
			}
		}
		if choice < 0 {
			for i, o := range d.Options {
				if o.Kind == "pass" {
					choice = i
					break
				}
			}
		}
		if choice < 0 {
			t.Fatalf("decision has neither a cast nor a pass option: %+v", d)
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}
		intents = append(intents, in)
		if err := g.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", n, err)
		}
		if tr := g.E.ChanceTranscript(); len(tr) > before {
			if !cast {
				t.Fatalf("a draw appeared before the coin spell was cast: %v", tr[before:])
			}
			return intents, tr[before].Value
		}
	}
	t.Fatal("coin spell did not resolve")
	return nil, 0
}
