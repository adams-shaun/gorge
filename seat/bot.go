package seat

import (
	"context"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Bot is a deterministic policy with its own RNG, independent of the
// engine's (rules/rng.go), so a match is reproducible from (engine seed, bot
// seed). It picks from the options the engine offered and nothing else,
// which is the same contract a human client has (Ruling P8).
//
// rules/fuzz_test.go's testBot (rules/testbot_test.go) is a line-for-line
// mirror of botDecide below: the rules package cannot import seat without
// running the dependency order backwards (Ruling F7), so the rules fuzz
// test carries its own copy instead. Keep the two in step.
type Bot struct {
	r *rand.Rand
}

// NewBot seeds the bot's own PCG source. Never math/rand's global functions
// and never the engine's rng: a match's outcome must be a pure function of
// (engine seed, bot seed), nothing else.
func NewBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// Decide answers d with an aggro-leaning policy (botDecide below). ctx and v
// are unused: the bot needs no rules knowledge and no state beyond the
// options offered -- unlike a human seat, which is the whole reason Decide
// takes them at all (Ruling P8).
func (b *Bot) Decide(_ context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	return botDecide(&d, b.r), nil
}

// botDecide implements every decision.Kind the engine (or a future one) can
// ask, so the bot needs no rules knowledge either (Ruling P7):
//
//   - KPriority: tap for mana, then make a land drop, then cast, then pass.
//   - KTarget: prefer an opposing player; fall back to the first legal
//     target.
//   - KAttackers: attack with everything that can.
//   - KBlockers: block with roughly half the legal (blocker, attacker)
//     pairs, at most once per blocker -- used is a membership set only,
//     ranged never, so it does not reach the chosen order.
//   - KTriggerOrder: a permutation of [0, n) drawn from the bot's own rng,
//     so ordering paths get fuzz coverage too.
//   - KTriggerOptional: a coin from the bot's own rng between the two
//     offered options ("yes" first, "no" second, per askTriggerOptional),
//     so both branches get coverage.
//
// Anything else -- KMulligan, KModes, any kind added later, and any case
// above that found nothing to pick -- falls to the last resort: pass if one
// is offered and Min == 0, otherwise the first Min distinct option indices.
// Decision.Validate demands exactly Min..Max distinct in-range choices, so
// this function must never return anything else, and every access into
// d.Options is guarded against the list being empty.
func botDecide(d *decision.Decision, r *rand.Rand) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		for _, want := range [...]string{"activate", "play_land", "cast"} {
			for _, o := range d.Options {
				if o.Kind == want {
					in.Choices = []int{o.Index}
					return in
				}
			}
		}

	case decision.KTarget:
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player != d.Player {
				in.Choices = []int{o.Index}
				return in
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index}
			return in
		}

	case decision.KAttackers:
		ch := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			ch = append(ch, o.Index)
		}
		in.Choices = ch
		return in

	case decision.KBlockers:
		var ch []int
		used := map[state.ObjID]bool{} // membership only -- never ranged.
		for _, o := range d.Options {
			if !used[o.Obj] && r.IntN(2) == 0 {
				used[o.Obj] = true
				ch = append(ch, o.Index)
			}
		}
		in.Choices = ch
		return in

	case decision.KTriggerOrder:
		if n := len(d.Options); n > 0 {
			perm := make([]int, n)
			for i := range perm {
				perm[i] = i
			}
			for i := n - 1; i > 0; i-- {
				j := r.IntN(i + 1)
				perm[i], perm[j] = perm[j], perm[i]
			}
			in.Choices = perm
			return in
		}

	case decision.KTriggerOptional:
		if idx := r.IntN(2); idx < len(d.Options) {
			in.Choices = []int{d.Options[idx].Index}
			return in
		}
	}

	// Last resort (Ruling P7): pass if Min == 0 and one is offered, else the
	// first Min distinct indices.
	if d.Min == 0 {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return in
			}
		}
		return in
	}
	n := d.Min
	if n > len(d.Options) {
		n = len(d.Options)
	}
	ch := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ch = append(ch, d.Options[i].Index)
	}
	in.Choices = ch
	return in
}
