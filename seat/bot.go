package seat

import (
	"context"
	"math/rand/v2"
	"strings"

	"github.com/adams-shaun/gorge/botpolicy"
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
		IsMain:    v.Phase == "main1" || v.Phase == "main2",
		Creatures: make(map[state.ObjID]botpolicy.Creature, 32),
		Life:      make(map[state.PlayerID]int32, len(v.Players)),
	}
	for _, p := range v.Players {
		b.Life[p.ID] = p.Life
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
	}
	return b
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
