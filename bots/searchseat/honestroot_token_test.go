package searchseat_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestHonestRootAtATokenAuraCopyEnchantChoice replays the one G0 hands-bot
// gate game whose az seat lost its honest root (draftrl gate -seed 201
// -events 24 -games 4, hands-bot#gih drafters; game seed
// 5993994771396169689, hero e203-s1 vs FDN06-RG, game 1). At decision seq
// 2315 (2211 in the original tape) seat 1 creates a token copy of its Imprisoned in the Moon and the
// token asks what it enchants (CR 303.4f) before it enters: the staged
// token is in no zone, so its Zone field reads the zero value ZLibrary, and
// the redealer took the decision's Source for an unpinned hidden library
// card ("observed frame refers to an unpinned hidden card"). A token is
// never a hidden card (CR 111.8); the root must build.
//
// The tape (testdata/token-aura-copy-tape.json, card names and intents
// only -- no Forge script text) was written by draftrl gate -only-game
// 5993994771396169689 -only-game-tape.
//
// The tape holds 478 intents, each matched to its decision by (turn, step,
// player, kind), and the enchant choice is at decision seq 2315. A rules change
// that adds a priority window means re-deriving it, inserting a pass at each
// new window.
func TestHonestRootAtATokenAuraCopyEnchantChoice(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	raw, err := os.ReadFile("testdata/token-aura-copy-tape.json")
	if err != nil {
		t.Fatal(err)
	}
	var tape struct {
		Seed      uint64            `json:"seed"`
		Names     [2]string         `json:"names"`
		Mulligans int               `json:"mulligans"`
		MaxObjs   int               `json:"max_objs"`
		Decks     [2][]string       `json:"decks"`
		Intents   []decision.Intent `json:"intents"`
	}
	if err := json.Unmarshal(raw, &tape); err != nil {
		t.Fatal(err)
	}
	decks := make([][]*cards.Card, 2)
	for i, names := range tape.Decks {
		for _, n := range names {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("deck %d: %q not in the corpus", i, n)
			}
			decks[i] = append(decks[i], c)
		}
	}
	cfg := rules.Config{
		Seed: tape.Seed, Names: tape.Names[:], Decks: decks, Tokens: reg.Tokens,
		NameUniverse: reg.Universe(), Mulligans: tape.Mulligans,
		LoopGuard: &rules.LoopGuard{MaxObjs: tape.MaxObjs},
	}
	const actor, seq = state.PlayerID(1), uint64(2315)
	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	e.Advance()
	feed := searchseat.NewFeed(actor)
	setup := searchprobe.PublicGame{Names: cfg.Names, Decks: cfg.Decks, Tokens: cfg.Tokens, StartingLife: cfg.StartingLife}
	for i, in := range tape.Intents {
		d := e.Pending()
		if d == nil {
			t.Fatalf("intent %d: no pending decision", i)
		}
		if _, ok := feed.Observe(e); !ok {
			t.Fatalf("intent %d: feed stopped: %s", i, feed.StopReason())
		}
		if d.Player == actor && d.Seq == seq {
			root, reason := searchseat.HonestRoot(setup, feed, e, bots.RootSeed(cfg.Seed^(uint64(actor)+1), d.Seq))
			if root == nil {
				t.Fatalf("decision seq %d (%s): honest root refused: %s", d.Seq, d.Kind, reason)
			}
			return
		}
		if d.Player == actor {
			if in.Payment != nil || in.Announce != nil {
				e.EnsurePaymentActions()
			}
			if err := feed.RecordAnswer(d, in); err != nil {
				t.Fatalf("intent %d: record answer: %v", i, err)
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d (seq %d): submit: %v", i, d.Seq, err)
		}
	}
	t.Fatalf("the tape never reached seat %d's decision seq %d", actor, seq)
}
