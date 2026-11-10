package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestControlPlayerRefusedByDesign pins kgen's deliberate refusal of
// AB$ ControlPlayer (CR 720/723 player control). gorge models the interval in
// engine/effects/choose_control.go effControlPlayer (an events.
// ControlPlayerChange folded into Game.ControlledBy/ControlArmedTurn/
// ControlCombatOnly, read back by Game.LiveController in
// engine/state/game.go), and redirects every decision the controlled player
// would be asked to the controller in engine/rules/engine_host.go
// controlPlayerRedirect: Decision.Player becomes the controller while
// Decision.Actor stays the controlled subject. dirac has no such split --
// Decision.Player (dirac/types.go) is a uint8 with no actor field, and every
// offer builder in gen.go/turn.go assumes the answerer is the actor -- while
// the lockstep oracle compares Decision.Player (dirac/oracle/lockstep.go:416).
// A lowered ability would therefore diverge on the answering seat; modelling
// it needs an answerer/actor split plus per-seat control state within the
// state-size gates, which is a separate design, so kgen refuses it by name.
func TestControlPlayerRefusedByDesign(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lowerNamed := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}

	// Every corpus card carrying ControlPlayer stays refused, so the refusal
	// cannot silently disappear when another blocker is fixed. Two are named
	// in dirac set lists (The Dominion Bracelet, Construct a Cosmic Cube); the
	// rest are outside the lowered card pool. Cruel Entertainment refuses at
	// its earlier ChoosePlayer node and Urza, Academy Headmaster at
	// GenericChoice, so their ControlPlayer svar is never reached; the other
	// seven must name the api.
	names := []string{
		"Worst Fears",
		"Mindslaver",
		"Emrakul, the Promised End",
		"Cruel Entertainment",
		"Construct a Cosmic Cube",
		"Urza, Academy Headmaster",
		"The Dominion Bracelet",
		"Secret of Bloodbending",
		"Sorin Markov",
	}
	shadowed := map[string]string{
		"Cruel Entertainment":      "ChoosePlayer",
		"Urza, Academy Headmaster": "GenericChoice",
	}
	found := 0
	for _, name := range names {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			continue // a corpus slice without the set: not present, not refused
		}
		found++
		got := keclReason(lowerNamed(name).Refusals)
		if got == "" {
			t.Errorf("%s: lowers whole, but ControlPlayer has no dirac model", name)
			continue
		}
		if why, ok := shadowed[name]; ok {
			if !strings.Contains(got, why) {
				t.Errorf("%s: refusals %q, want the earlier %s blocker", name, got, why)
			}
			continue
		}
		if !strings.Contains(got, "api ControlPlayer") {
			t.Errorf("%s: refusals %q, want the planted api ControlPlayer refusal", name, got)
		}
	}
	if found == 0 {
		t.Fatal("the corpus registry carries none of the nine ControlPlayer cards")
	}

	// A bare activated ability with no other blocker refuses exactly at the
	// api, not earlier on its cost or targets.
	const probe = "Name:Control Player Probe\nManaCost:1\nTypes:Sorcery\n" +
		"A:AB$ ControlPlayer | Cost$ 1 | ValidTgts$ Opponent | SpellDescription$ probe\n"
	card, diags := cards.ParseBytes("controlplayer_probe.txt", []byte(probe))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const pname = "Control Player Probe"
	p := &pool{names: []string{pname}, byName: map[string]*cards.Card{pname: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, pname, card, false)
	// kgen prefixes an ability's refusal with its node; the API fragment is
	// the whole reason, with no earlier cost or target refusal before it.
	if len(lc.Refusals) != 1 || !strings.HasSuffix(lc.Refusals[0].Reason, ": api ControlPlayer") {
		t.Fatalf("synthetic ControlPlayer: refusals %v, want exactly the api ControlPlayer refusal", lc.Refusals)
	}
}
