package gamecfg_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

// TestChancePrefixSuppliesCoinFlip builds a game whose genesis is replayed from
// a supplied prefix and whose next draw is a coin flip, not a shuffle: the
// prefix value is consumed and CheckRandomness passes, while the same game
// without a prefix reports the flip as unplanned randomness.
func TestChancePrefixSuppliesCoinFlip(t *testing.T) {
	reg := testcorpus.Registry(t)
	coin := chanceCoinCard(t)
	deckCards := make([]*cards.Card, 12)
	for i := range deckCards {
		deckCards[i] = coin
	}
	sec := secret(21)
	decks := [2][]*cards.Card{deckCards, deckCards}

	baseline, err := gamecfg.New(reg, sec, decks, gamecfg.Rules{Mulligan: "none"})
	if err != nil {
		t.Fatal(err)
	}
	genesis := append([]rules.ChanceDraw(nil), baseline.E.ChanceTranscript()...)
	if len(genesis) == 0 || genesis[0].Bound != 2 {
		t.Fatalf("precondition: genesis transcript does not start with the toss: %v", genesis)
	}
	intents, defaultFlip := castFirstCoinFlip(t, baseline, len(genesis))
	if len(baseline.E.ChanceTranscript()) != len(genesis)+1 {
		t.Fatalf("precondition: coin flip drew %d values, want 1",
			len(baseline.E.ChanceTranscript())-len(genesis))
	}

	// The supplied value differs from what the seeded generator would draw, so
	// consuming the prefix is observable even if the generator is unchanged.
	supplied := 1 - defaultFlip
	prefix := append(append([]rules.ChanceDraw(nil), genesis...),
		rules.ChanceDraw{Bound: 2, Value: supplied})
	g, err := gamecfg.New(reg, sec, decks, gamecfg.Rules{Mulligan: "none", ChancePrefix: prefix})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.CheckRandomness(); err != nil {
		t.Fatalf("genesis prefix left an unplanned draw: %v", err)
	}
	for i, in := range intents {
		if err := g.Submit(in); err != nil {
			t.Fatalf("replay intent %d: %v", i, err)
		}
	}
	got := g.E.ChanceTranscript()
	if len(got) != len(prefix) {
		t.Fatalf("transcript has %d draws, want the %d-draw prefix", len(got), len(prefix))
	}
	if got[len(got)-1] != (rules.ChanceDraw{Bound: 2, Value: supplied}) {
		t.Fatalf("coin flip = %v, want the supplied %d (generator would give %d)",
			got[len(got)-1], supplied, defaultFlip)
	}
	if err := g.CheckRandomness(); err != nil {
		t.Fatalf("prefix-supplied flip counted as unplanned: %v", err)
	}
}

// TestChancePrefixNilUnchanged pins the default: with no prefix the forced
// starting-seat toss and the secret-derived shuffle plan still build the game.
func TestChancePrefixNilUnchanged(t *testing.T) {
	reg := testcorpus.Registry(t)
	burn := deck(t, reg, "Burn")
	g, err := gamecfg.New(reg, secret(5), [2][]*cards.Card{burn, burn},
		gamecfg.Rules{Mulligan: "none", StartingSeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	if g.E.G.StartingPlayer != 1 {
		t.Fatalf("starting player %d, want the forced seat 1", g.E.G.StartingPlayer)
	}
	tr := g.E.ChanceTranscript()
	if len(tr) == 0 || tr[0] != (rules.ChanceDraw{Bound: 2, Value: 1}) {
		t.Fatalf("first draw = %v, want the forced toss {2 1}", tr)
	}
	if err := g.CheckRandomness(); err != nil {
		t.Fatalf("default game reported unplanned randomness: %v", err)
	}
}

// chanceCoinCard is a zero-cost sorcery whose only effect is one coin flip.
func chanceCoinCard(t testing.TB) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("chance_coin.txt", []byte(
		"Name:Chance Coin\nManaCost:0\nTypes:Sorcery\n"+
			"A:SP$ FlipCoin | WinSubAbility$ DBGain\n"+
			"SVar:DBGain:DB$ GainLife | Life$ 1\n"+
			"Oracle:Flip a coin.\n"))
	if len(diags) != 0 {
		t.Fatalf("card diags: %v", diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// castFirstCoinFlip drives g to the first draw past before, casting the coin
// spell once and passing afterwards. The flip is the first draw no prefix or
// planner supplied, so the submit that records it returns
// ErrUnplannedRandomness. It returns the intents submitted and the recorded
// flip value.
func castFirstCoinFlip(t *testing.T, g *gamecfg.Game, before int) ([]decision.Intent, int) {
	t.Helper()
	var intents []decision.Intent
	cast := false
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
		err := g.Submit(in)
		tr := g.E.ChanceTranscript()
		if len(tr) > before {
			if !cast {
				t.Fatalf("an unplanned draw appeared before the coin spell was cast: %v", tr[before:])
			}
			if !errors.Is(err, gamecfg.ErrUnplannedRandomness) {
				t.Fatalf("unplanned coin flip error = %v, want %v", err, gamecfg.ErrUnplannedRandomness)
			}
			return intents, tr[before].Value
		}
		if err != nil {
			t.Fatalf("intent %d: %v", n, err)
		}
	}
	t.Fatal("coin spell did not resolve")
	return nil, 0
}
