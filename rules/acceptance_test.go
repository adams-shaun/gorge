package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// knownUnsupported is the M1 coverage RATCHET (Ruling P12/D2-a): the exact
// card -> missing-primitive set TestEveryRepoDeckIsFullySupported measured
// on its first run against the 12 repo decks, checked in by hand. The test
// below asserts the MEASURED set equals this table EXACTLY, in both
// directions -- a newly-missing card is a regression, a table entry that is
// now fully supported is stale and must be deleted -- so this table only
// ever shrinks, and only by implementing a real primitive (Ruling W2: this
// project does not grow the "supported" set just to make the ratchet green).
//
// Measured 2026-09-04 against the compiled IR cache at .cards/ir.gob.gz
// (corpus master @ 95f04e8a04c8925fa97cb226fc3341cabcc90a53): 35 of the 136
// distinct cards across the 12 decks need at least one primitive this build
// does not implement -- overwhelmingly individual keywords (kw:Equip,
// kw:Flash, kw:Kicker, kw:Delve, kw:Undying, kw:etbCounter, kw:Storm, and so
// on) rather than whole missing APIs, since Task 26 lands after the API/
// trigger/static/replacement families that gate the most cards (see the
// forgec report's top-missing list). Every unimplemented ability on these
// cards is inert for the acceptance run (Ruling U13's Sword of Fire and Ice
// note is this same shape, one card up): the point of Task 26 is that the
// games terminate, invariants hold and replay is exact with these cards
// shuffled in, not that every card plays with full fidelity yet -- that is
// M4's coverage work, and this table is its worklist.
var knownUnsupported = map[string][]string{
	"Batterskull":                  {"kw:Equip", "kw:Living Weapon"},
	"Cabal Therapy":                {"kw:Flashback"},
	"Cavern of Souls":              {"kw:ETBReplacement"},
	"Chain Lightning":              {"api:CopySpellAbility"},
	"Chalice of the Void":          {"kw:etbCounter"},
	"Empty the Warrens":            {"api:Token", "kw:Storm"},
	"Endless One":                  {"kw:etbCounter"},
	"Entreat the Angels":           {"api:Token", "kw:Miracle"},
	"Experiment One":               {"kw:Evolve"},
	"Gatekeeper of Malakir":        {"kw:Kicker"},
	"Geralf's Messenger":           {"kw:Undying"},
	"Goblin Bushwhacker":           {"kw:Kicker"},
	"Goblin Piledriver":            {"kw:Protection from blue"},
	"Gurmag Angler":                {"kw:Delve"},
	"Knight of Infamy":             {"kw:Exalted", "kw:Protection from white"},
	"Monastery Swiftspear":         {"kw:Prowess"},
	"Phyrexian Revoker":            {"kw:ETBReplacement"},
	"Pithing Needle":               {"kw:ETBReplacement"},
	"Rancor":                       {"kw:Enchant"},
	"Reckless Bushwhacker":         {"kw:Surge"},
	"Sanctum Prelate":              {"kw:ETBReplacement"},
	"Snapcaster Mage":              {"kw:Flash"},
	"Spectral Sailor":              {"kw:Flash"},
	"Strangleroot Geist":           {"kw:Undying"},
	"Sword of Fire and Ice":        {"kw:Equip"},
	"Tendrils of Agony":            {"kw:Storm"},
	"Terminus":                     {"kw:Miracle"},
	"Tombstalker":                  {"kw:Delve"},
	"Ulamog, the Ceaseless Hunger": {"kw:Indestructible"},
	"Umezawa's Jitte":              {"kw:Equip"},
	"Vines of Vastwood":            {"kw:Kicker"},
	"Walking Ballista":             {"kw:etbCounter"},
	"World Breaker":                {"kw:Devoid"},
	"Wurmcoil Engine":              {"api:Token"},
	"Young Pyromancer":             {"api:Token"},
}

// TestEveryRepoDeckIsFullySupported is the M1 coverage ratchet: every card
// in the 12 Legacy decks is either fully playable, or is named in
// knownUnsupported with the exact primitives it is missing. A card missing
// something knownUnsupported does not list is a regression (fails); a card
// knownUnsupported lists that is now fully supported is a stale entry that
// must be deleted from the table (also fails) -- the table can drift out of
// sync in either direction as the engine grows, and both directions are a
// bug in the ratchet, not something to silently tolerate.
func TestEveryRepoDeckIsFullySupported(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	supported := effects.Supported()
	measured := map[string][]string{}
	total := 0
	seen := map[string]bool{}
	for _, name := range testutil.RepoDeckNames() {
		for _, c := range testutil.RepoDeck(t, reg, name) {
			cardName := c.Faces[0].Name
			if seen[cardName] {
				continue
			}
			seen[cardName] = true
			total++
			if m := reg.Unsupported(c, supported); len(m) > 0 {
				measured[cardName] = m
			}
		}
	}
	t.Logf("ratchet: %d of %d distinct cards across the repo decks are not fully supported",
		len(measured), total)

	for card, gotPrims := range measured {
		want, ok := knownUnsupported[card]
		if !ok {
			t.Errorf("%s needs %v, which is not in knownUnsupported -- new gap, add it to the ratchet table", card, gotPrims)
			continue
		}
		if !sameSet(want, gotPrims) {
			t.Errorf("%s: knownUnsupported says %v, measured %v -- update the ratchet table to match", card, want, gotPrims)
		}
	}
	for card, want := range knownUnsupported {
		if _, stillMissing := measured[card]; !stillMissing {
			t.Errorf("%s is fully supported now (was missing %v) -- delete it from knownUnsupported", card, want)
		}
	}
}

// sameSet reports whether a and b hold the same strings, order-independent
// and duplicate-independent -- cards.Card.Primitives (and therefore
// Registry.Unsupported) already returns a sorted, deduplicated slice, so in
// practice this only ever needs to reject a genuine content difference, not
// tolerate reordering, but checking it this way keeps the comparison
// correct even if that guarantee ever loosens.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := map[string]int{}
	for _, s := range a {
		am[s]++
	}
	for _, s := range b {
		am[s]--
	}
	for _, n := range am {
		if n != 0 {
			return false
		}
	}
	return true
}

// TestRepoDecksPlayAtEverySeatCount is the M1 acceptance gate: the 12 repo
// decks, round-robined across 2, 4, 6 and 8 seats, must each play a
// complete game -- termination under budget, every invariant holding
// throughout -- with the cards knownUnsupported lists still shuffled in and
// simply inert (Ruling: this task is about the engine surviving real Forge
// data at scale, not about every card's fidelity, which is M4's separate
// worklist).
func TestRepoDecksPlayAtEverySeatCount(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	all := testutil.RepoDeckNames()
	for _, seats := range []int{2, 4, 6, 8} {
		names := make([]string, seats)
		decks := make([][]*cards.Card, seats)
		for i := 0; i < seats; i++ {
			names[i] = all[i%len(all)]
			decks[i] = testutil.RepoDeck(t, reg, all[i%len(all)])
		}
		e := New(Config{Seed: 42, Names: names, Decks: decks, Tokens: reg.Tokens})
		b := newTestBot(7)
		e.Advance()
		testutil.CheckInvariants(t, e.G, e.Pending(), fmt.Sprintf("%d-seat start", seats))
		n := 0
		for !e.G.Over && e.Pending() != nil && n < 400000 {
			// Ruling T25-b's isMain gate (rules/testbot_test.go's own doc):
			// computed from the engine's own step, on the line before the
			// call that consumes it, exactly like TestInvariantsUnderSeedFuzz.
			isMain := e.G.Step.IsMain()
			if err := e.Submit(b.answer(isMain, e.Pending())); err != nil {
				t.Fatalf("%d seats, intent %d: %v", seats, n, err)
			}
			if n%997 == 0 {
				testutil.CheckInvariants(t, e.G, e.Pending(), fmt.Sprintf("%d-seat mid", seats))
			}
			n++
		}
		testutil.CheckInvariants(t, e.G, e.Pending(), fmt.Sprintf("%d-seat end", seats))
		if !e.G.Over {
			t.Fatalf("%d-seat game did not finish (turn %d, %d intents)", seats, e.G.Turn, n)
		}
		// Ruling P14: Draw before Winner -- Winner's zero value is seat 0, a
		// real seat, so reading it unconditionally would misreport a drawn
		// game as "seat 0 won".
		result := "draw"
		if !e.G.Draw {
			result = e.G.Players[e.G.Winner].Name
		}
		t.Logf("%d seats: %6d intents, %6d events, %3d turns, winner=%s, chain=%s",
			seats, n, len(e.L.Events), e.G.Turn, result, e.L.Head())
	}
}

// TestRepoDeckGamesReplayExactly ties acceptance to the replay guarantee:
// five seeded 4-seat games over the repo decks, each re-run from its own
// recorded (Config, Log) through the package-local replayFor helper, must
// reach the same chain Head as the original run.
func TestRepoDeckGamesReplayExactly(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	all := testutil.RepoDeckNames()
	for seed := uint64(0); seed < 5; seed++ {
		names := make([]string, 4)
		decks := make([][]*cards.Card, 4)
		for i := 0; i < 4; i++ {
			names[i] = all[(int(seed)+i)%len(all)]
			decks[i] = testutil.RepoDeck(t, reg, names[i])
		}
		cfg := Config{Seed: seed, Names: names, Decks: decks, Tokens: reg.Tokens}
		e := New(cfg)
		b := newTestBot(seed)
		e.Advance()
		n := 0
		for !e.G.Over && e.Pending() != nil && n < 400000 {
			isMain := e.G.Step.IsMain()
			if err := e.Submit(b.answer(isMain, e.Pending())); err != nil {
				t.Fatalf("seed %d, intent %d: %v", seed, n, err)
			}
			n++
		}
		if !e.G.Over {
			t.Fatalf("seed %d did not terminate after %d intents (turn %d)", seed, n, e.G.Turn)
		}
		re, err := replayFor(cfg, e.L)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if re.L.Head() != e.L.Head() {
			t.Fatalf("seed %d chain %s, replay %s", seed, e.L.Head(), re.L.Head())
		}
		t.Logf("seed %d: %d intents, chain %s, replay OK", seed, n, e.L.Head())
	}
}

// replayFor is a thin, package-local mirror of replay.Replay (Task 24):
// this test file is package rules and the replay package imports rules, so
// importing it here would be a cycle (Ruling in the supplement's ¶0). It
// reruns l's recorded Intents against a fresh Engine built from cfg -- the
// same (Config, Log) contract replay.Replay documents -- and returns the
// resulting engine; the replay package's own exported API and its own
// tests are what every other caller uses.
func replayFor(cfg Config, l *events.Log) (*Engine, error) {
	cfg.Seed = l.Seed
	e := New(cfg)
	e.Advance()
	for i := 0; i < len(l.Intents); i++ {
		if e.G.Over {
			break
		}
		if e.Pending() == nil {
			return e, fmt.Errorf("replayFor: no decision pending at intent %d", i)
		}
		if err := e.Submit(l.Intents[i]); err != nil {
			return e, fmt.Errorf("replayFor: intent %d rejected: %w", i, err)
		}
	}
	return e, nil
}
