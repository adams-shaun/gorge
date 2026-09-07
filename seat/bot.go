package seat

import (
	"context"
	"math/rand/v2"
	"strings"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Bot is a deterministic policy with its own RNG, independent of the
// engine's (rules/rng.go), so a match is reproducible from (engine seed, bot
// seed). It picks from the options the engine offered and nothing else,
// which is the same contract a human client has (Ruling P8).
//
// The policy itself lives in botpolicy (botpolicy/policy.go), shared with
// the rules package's fuzz testBot (rules/testbot_test.go): rules cannot
// import seat without running the dependency order backwards (Ruling F7),
// so both sides build a botpolicy.Board from what they can see and call the
// same botpolicy.Decide. This file is the view-shaped half of that -- it
// converts the projected View into the Board and seeds the bot's own rng.
// There is no second copy of the policy to keep in step; seat/integration_test.go's
// TestBotAdaptersAgree* pins the two halves to the same Board for the same
// game facts.
type Bot struct {
	r *rand.Rand
}

// M4: a compile-time assertion that Bot keeps satisfying Seat, since
// seat.go and bot.go otherwise never reference each other.
var _ Seat = (*Bot)(nil)

// M4: Bot also satisfies BoardSeat — the game-shaped half of the adapter
// pair, answered without a projected View. host builds the botpolicy.Board
// (via BoardFromGame, the same Board boardFromView would lift off the View)
// under the match's exclusive lock and calls this instead of Decide, so a
// bot seat never forces cardViews' string round-trip.
var _ BoardSeat = (*Bot)(nil)

// NewBot seeds the bot's own PCG source. Never math/rand's global functions
// and never the engine's rng: a match's outcome must be a pure function of
// (engine seed, bot seed), nothing else.
func NewBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// Decide answers d with the combat-aware policy in botpolicy. v is read
// for two things -- whether it is currently a main phase, and the public
// battlefield/life facts the combat heuristic reads (both halves of
// boardFromView below) -- the reason Decide takes a View at all rather
// than acting on d alone. rules/testbot_test.go's testBot has no View and
// gets the same facts from the engine (e.G.Step.IsMain, botpolicy.BoardFromGame)
// instead -- the game-shaped half of the same adapter pair;
// TestBotAdaptersAgree* (integration_test.go) pins the two halves to the
// same Board for the same game facts.
func (b *Bot) Decide(_ context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return botpolicy.Decide(boardFromView(v), &d, b.r), nil
}

// DecideBoard is the game-shaped half of Decide: the Board is already built
// (under the match lock, from botpolicy.BoardFromGame reading state.Game and
// the engine's derived P/T/keywords) and handed in as a value, so the bot
// answers without the view->Board string round-trip. The engine the Board
// was built from derives exactly the facts the projected View would have
// carried (TestBotAdaptersAgreeOverWholeGame pins the two halves).
func (b *Bot) DecideBoard(_ context.Context, brd botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	return botpolicy.Decide(brd, &d, b.r), nil
}

// boardFromView is the view-shaped adapter: the Board the policy reads,
// lifted off the projected View a real client would receive. The combat
// half (Creatures, Life) is every public battlefield creature and life
// total the seat can see -- exactly the facts botpolicy.BoardFromGame
// derives from the engine's state.Game (same zones, same derived P/T and
// keywords, same filters: cardless ability objects and off-battlefield
// ephemerals are dropped by the View itself, and the creature test below
// mirrors cards/face.go's hasType membership check on the joined type
// list). The rules test host computes the same Board from the engine;
// seat/integration_test.go's TestBotAdaptersAgreeOverWholeGame pins the
// two halves to the same facts over a whole game.
func boardFromView(v view.View) botpolicy.Board {
	b := botpolicy.Board{
		IsMain:     v.Phase == "main1" || v.Phase == "main2",
		Creatures:  make(map[state.ObjID]botpolicy.Creature, 32),
		Life:       make(map[state.PlayerID]int32, len(v.Players)),
		Cards:      make(map[state.ObjID]botpolicy.Card, 16),
		Commanders: make(map[state.ObjID]botpolicy.Commander, 8),
	}
	for _, p := range v.Players {
		b.Life[p.ID] = p.Life
		if p.ID == v.Viewer {
			// The tap gate's pool (tap.go): the projecting viewer's own mana
			// pool, the same numbers poolView lifted off the engine state
			// that BoardFromGameInto reads directly, so the two halves agree
			// (pinned non-vacuously by integration_test.go's pool agreement).
			for sym, n := range p.Pool {
				b.Pool[state.ManaIndex(sym[0])] = n
			}
		}
		for _, cv := range p.Battlefield {
			if !isCreatureView(cv) {
				continue
			}
			b.Creatures[cv.ID] = botpolicy.Creature{
				Power:      cv.Power,
				Toughness:  cv.Toughness,
				Damage:     cv.Damage,
				Keywords:   cv.Keywords,
				Tapped:     cv.Tapped,
				Controller: cv.Controller,
			}
		}
		// The commander bookkeeping: the projected roster (p.Commanders,
		// never shrunk as commanders are cast) carries identity and the CR
		// 903.8 cast counts; p.Command's zone-list membership is
		// InCommandZone's exact mirror of the game half's
		// g.Zone(ZCommand, owner) read; every player's CmdDamage re-keys
		// the CR 903.10 clock by commander object id exactly as the game
		// half's dense-index transpose does.
		for k, cv := range p.Commanders {
			var casts int32
			if k < len(p.CommanderCasts) {
				casts = p.CommanderCasts[k]
			}
			cmdr := botpolicy.Commander{Casts: casts}
			for _, cz := range p.Command {
				if cz.ID == cv.ID {
					cmdr.InCommandZone = true
					break
				}
			}
			b.Commanders[cv.ID] = cmdr
		}
	}
	// The CR 903.10 clock, filled from every player's damage keys. The map
	// iteration order is irrelevant: each entry lands in b.Commanders[id]'s
	// own Damage map keyed by the player who took it, and no policy branch
	// reads anything in order this fill could disturb (ties break on ObjID
	// or option index). A commander a damaged player names is always
	// already in b.Commanders (damage only ever accrues to a commander) —
	// the nil-map read would be a zero entry otherwise, never a crash.
	for _, q := range v.Players {
		for id, tally := range q.CmdDamage {
			cmdr := b.Commanders[id]
			if cmdr.Damage == nil {
				cmdr.Damage = make(map[state.PlayerID]int32, 2)
			}
			cmdr.Damage[q.ID] = tally
			b.Commanders[id] = cmdr
		}
	}
	// The casting Card census: the viewer's own hand, graveyard, battlefield
	// and command-zone CardViews — the deciding seat's own legally-seen
	// zones — mirroring exactly what BoardFromGame fills from state.Game for
	// that seat (cast.go's CmcOf and the type-word check on the same printed
	// fields, and the engine's derived Power that the View already projects
	// as cv.Power), so the casting policy ranks the same card the same way
	// on both halves. The fill is per zone, because the tap gate (tap.go)
	// reads Castable from the zone a card sits in: a hand or command-zone
	// card is worth mana (its cast is offered as soon as the pool pays the
	// cost; a command-zone card is a commander the CR 903.8 tax prices), a
	// graveyard card only when the View's derived keyword list carries
	// Flashback (the same Derived list the engine's own flashback gate
	// reads), and a battlefield permanent never.
	for _, p := range v.Players {
		if p.ID != v.Viewer {
			continue
		}
		fillZone := func(zone []view.CardView, castable func(view.CardView) bool, battlefield bool) {
			for _, cv := range zone {
				var produces cards.ManaProduction
				if cv.Produces != nil {
					produces = *cv.Produces
				}
				b.Cards[cv.ID] = botpolicy.Card{
					Creature:      isCreatureView(cv),
					Power:         cv.Power,
					CMC:           botpolicy.CmcOf(cv.ManaCost),
					Basic:         hasBasicView(cv),
					AttachedTo:    cv.AttachedTo,
					ManaCost:      cv.ManaCost,
					Castable:      castable(cv),
					OnBattlefield: battlefield,
					Produces:      produces,
					InstantSpeed:  instantSpeedView(cv),
				}
			}
		}
		aCastable := func(view.CardView) bool { return true }
		notCastable := func(view.CardView) bool { return false }
		fillZone(p.Hand, aCastable, false)
		fillZone(p.Graveyard, hasFlashbackView, false)
		fillZone(p.Battlefield, notCastable, true)
		fillZone(p.Command, aCastable, false)
	}
	return b
}

// hasFlashbackView is the view-shaped half of the tap gate's
// graveyard-castability test (botpolicy.combat.go's game-shaped half): the
// projected keyword list is the engine's Derived() output, so checking the
// card's keywords for "Flashback" (cards.KeywordHead-stripped, the same
// head test the game half runs) agrees with the engine's own flashback
// gate read.
func hasFlashbackView(cv view.CardView) bool {
	for _, k := range cv.Keywords {
		if strings.EqualFold(cards.KeywordHead(k), "Flashback") {
			return true
		}
	}
	return false
}

// isCreatureView is the view-shaped half of "is this battlefield object a
// creature": the game-shaped half reads the object's own Face().IsCreature
// (cards/face.go's hasType, an EqualFold membership check), and the View
// carries only the same type words joined into one string, so this mirrors
// exactly that check on the joined list.
func isCreatureView(cv view.CardView) bool {
	for _, t := range strings.Fields(cv.Types) {
		if strings.EqualFold(t, "Creature") {
			return true
		}
	}
	return false
}

// hasBasicView is the view-shaped half of "is this a basic land": the
// game-shaped half reads the face's own type list (BoardFromGame's
// Card.Basic via hasTypeWord), and the View carries the same type words
// joined into one string, so this mirrors exactly that check on the
// joined list.
func hasBasicView(cv view.CardView) bool {
	for _, t := range strings.Fields(cv.Types) {
		if strings.EqualFold(t, "Basic") {
			return true
		}
	}
	return false
}

// instantSpeedView is the view-shaped half of "can this card be cast at
// instant speed" (botpolicy.Card.InstantSpeed): the card is an Instant, or
// it carries the Flash keyword. The game-shaped half (BoardFromGame) reads
// the face's own type list and the engine's derived keyword list, which the
// View carries as cv.Types and cv.Keywords, so the two halves agree.
func instantSpeedView(cv view.CardView) bool {
	for _, t := range strings.Fields(cv.Types) {
		if strings.EqualFold(t, "Instant") {
			return true
		}
	}
	for _, k := range cv.Keywords {
		if strings.EqualFold(cards.KeywordHead(k), "Flash") {
			return true
		}
	}
	return false
}
