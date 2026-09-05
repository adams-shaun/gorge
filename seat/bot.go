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

// M4: a compile-time assertion that Bot keeps satisfying Seat, since
// seat.go and bot.go otherwise never reference each other.
var _ Seat = (*Bot)(nil)

// NewBot seeds the bot's own PCG source. Never math/rand's global functions
// and never the engine's rng: a match's outcome must be a pure function of
// (engine seed, bot seed), nothing else.
func NewBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// Decide answers d with an aggro-leaning policy (botDecide below). v is read
// for exactly one thing -- whether it is currently a main phase -- which is
// also the only reason Decide takes a View at all rather than acting on d
// alone (Ruling T25-b, fix round 1): tapping mana is only ever worth doing
// at sorcery speed, and a bot that taps everything the moment it gets
// priority (including during the upkeep, where the trigger drain holds it)
// empties its pool before main 1 and can never pay a cost above one land
// drop. rules/testbot_test.go's testBot has no View and reads
// e.G.Step.IsMain() instead -- see its own doc.
func (b *Bot) Decide(_ context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	isMain := v.Phase == "main1" || v.Phase == "main2"
	return botDecide(isMain, &d, b.r), nil
}

// botDecide implements every decision.Kind the engine (or a future one) can
// ask, so the bot needs no rules knowledge either (Ruling P7):
//
//   - KPriority: tap for mana only in a main phase (Ruling T25-b -- tapping
//     during the upkeep or combat empties the pool before there is anything
//     worth spending it on), then make a land drop, then cast, then pass.
//   - KTarget: prefer an opposing player; fall back to the first legal
//     target.
//   - KAttackers: attack with everything that can.
//   - KBlockers: block with roughly half the legal (blocker, attacker)
//     pairs, at most once per blocker -- used is a membership set only,
//     ranged never, so it does not reach the chosen order.
//   - KTriggerOrder: a permutation of the offered indices drawn from the
//     bot's own rng, so ordering paths get fuzz coverage too.
//   - KTriggerOptional: a coin from the bot's own rng between the two
//     offered options ("yes" first, "no" second, per askTriggerOptional),
//     so both branches get coverage.
//
// Anything else -- KMulligan, KModes, any kind added later, and any case
// above that found nothing to pick -- falls to the last resort: pass if one
// is offered and Min == 0, otherwise whatever clamp below tops up with.
//
// Ruling T25-c (fix round 1): every branch used to return its pick
// unclamped, so it only ever validated by coincidence of the Min/Max shapes
// today's engine happens to emit (rules/stack.go's askTarget already names
// TargetMin/TargetMax as coming). clamp is now the last thing every return
// does, so the totality guarantee holds by construction for any Min/Max the
// wire format allows, not only today's. Every access into d.Options remains
// guarded against the list being empty.
func botDecide(isMain bool, d *decision.Decision, r *rand.Rand) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		if isMain {
			for _, o := range d.Options {
				if o.Kind == "activate" {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		for _, want := range [...]string{"play_land", "cast"} {
			for _, o := range d.Options {
				if o.Kind == want {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}

	case decision.KTarget:
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player != d.Player {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index}
			return clamp(d, in)
		}

	case decision.KAttackers:
		ch := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			ch = append(ch, o.Index)
		}
		in.Choices = ch
		return clamp(d, in)

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
		return clamp(d, in)

	case decision.KTriggerOrder:
		if n := len(d.Options); n > 0 {
			// M3: build from o.Index like every other branch, not a bare
			// position literal -- identical today (Index == position
			// everywhere a real Decision is built, and invariant 5 checks
			// it), but this is the one branch that didn't say so.
			perm := make([]int, n)
			for i, o := range d.Options {
				perm[i] = o.Index
			}
			for i := n - 1; i > 0; i-- {
				j := r.IntN(i + 1)
				perm[i], perm[j] = perm[j], perm[i]
			}
			in.Choices = perm
			return clamp(d, in)
		}

	case decision.KTriggerOptional:
		if idx := r.IntN(2); idx < len(d.Options) {
			in.Choices = []int{d.Options[idx].Index}
			return clamp(d, in)
		}
	}

	// Last resort: pass if Min == 0 and one is offered; clamp below handles
	// everything else, including topping up to Min when nothing above (or
	// this) picked enough.
	if d.Min == 0 {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
	}
	return clamp(d, in)
}

// clamp enforces [Min, Max] on top of whatever botDecide's switch (or its
// fallback) picked (Ruling T25-c): truncate to at most Max, in the order
// already chosen, then -- if that leaves fewer than Min -- top up with the
// lowest-index unused options until Min is reached or none remain. This is
// the last thing every return in botDecide does, so Decision.Validate's
// Min..Max requirement holds for any shape the wire format allows, not only
// the ones reachable today.
func clamp(d *decision.Decision, in decision.Intent) decision.Intent {
	max := d.Max
	if max < 0 {
		max = 0
	}
	if len(in.Choices) > max {
		in.Choices = append([]int(nil), in.Choices[:max]...)
	}
	min := d.Min
	if min < 0 {
		min = 0
	}
	if len(in.Choices) < min {
		have := make(map[int]bool, len(in.Choices)) // membership only -- never ranged.
		for _, c := range in.Choices {
			have[c] = true
		}
		for _, o := range d.Options {
			if len(in.Choices) >= min {
				break
			}
			if !have[o.Index] {
				have[o.Index] = true
				in.Choices = append(in.Choices, o.Index)
			}
		}
	}
	return in
}
