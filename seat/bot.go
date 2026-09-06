package seat

import (
	"context"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
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

// Decide answers d with the aggro-leaning policy in botpolicy. v is read
// for exactly one thing -- whether it is currently a main phase -- which is
// also the only reason Decide takes a View at all rather than acting on d
// alone (Ruling T25-b, fix round 1): tapping mana is only ever worth doing
// at sorcery speed, and a bot that taps everything the moment it gets
// priority (including during the upkeep, where the trigger drain holds it)
// empties its pool before main 1 and can never pay a cost above one land
// drop. rules/testbot_test.go's testBot has no View and gets the same fact
// from e.G.Step.IsMain() instead -- the game-shaped half of the same
// adapter pair; TestBotAdaptersAgree* (integration_test.go) pins the two
// halves to the same Board for the same game facts.
func (b *Bot) Decide(_ context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return botpolicy.Decide(boardFromView(v), &d, b.r), nil
}

// boardFromView is the view-shaped adapter: the one board fact the policy
// reads, lifted off the projected View a real client would receive (the
// Phase string view.PhaseOf produces from the engine's step). The rules
// test host computes the same fact from e.G.Step.IsMain().
func boardFromView(v view.View) botpolicy.Board {
	return botpolicy.Board{IsMain: v.Phase == "main1" || v.Phase == "main2"}
}
