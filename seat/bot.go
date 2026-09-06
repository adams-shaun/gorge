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
//   - "concede" (M2d-3): never picked. It is another priority option kind,
//     served last after "pass", but no policy wants to leave the game it
//     is winning; the explicit kind scans below return before any blind
//     fallback, and clamp's top-up prefers "pass" (Ruling T25-g), which
//     legalActions (rules/legal.go) always offers. Unknown option kinds are
//     otherwise never chosen by this switch at all.
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
//   - KChoose: every option in one decision shares a Kind (Option.Kind, not
//     d.Kind) that says what is being chosen. "x" takes the highest option
//     (the most an {X} cost can pay for -- options ascend); "exile"/
//     "sacrifice" take the first Max options; "yes"/"no" always answers yes
//     (Kind "yes" is first, per how askers build the two-option list);
//     "name"/"type"/"number" take the first offer. No rng is consumed,
//     unlike KBlockers/KTriggerOrder/KTriggerOptional above.
//   - KMulligan: the London round (Config.Mulligans > 0) offers two shapes on
//     one kind. A bottoming decision (every option Kind "bottom", Min ==
//     Max == taken) bottoms the taken lowest-indexed cards -- Choices
//     [0,1,...,taken-1] in ascending index order. A keep/mulligan decision
//     mulligans with probability 1/3 off the bot's own rng when a
//     "mulligan" option is offered (the determinism mirror of
//     KTriggerOptional), otherwise keeps (the "keep" option at index 0). The
//     rng is consumed only where a real mulligan choice exists.
//   - KModes: the mid-resolution modal pick -- choose the first Min options
//     in order (Choices [0, 1, …, Min-1]), the recorded mirror of the
//     engine-side first-mode stand-in, no rng.
//
// Anything else — any kind added later, and any case
// above that found nothing to pick — falls to the last resort: pass if one
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
		// Task 10: in a main phase, when casting found nothing to do, take the
		// first "ability" option offered (legalActions only offers them as
		// legal sorcery-speed actions, so no extra isMain gate is needed here
		// beyond this block's own check). Replacements fall through to the
		// explicit pass below.
		if isMain {
			for _, o := range d.Options {
				if o.Kind == "ability" {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		// Ruling T25-g (fix round 2): explicitly pass here, before clamp
		// ever runs. This is the common case -- outside a main phase, or
		// with nothing affordable in one -- and legalActions
		// (rules/legal.go) lists every "activate" option before "pass", so
		// without this, clamp's blind first-unused-index top-up (needed
		// because a priority decision is always Min:1/Max:1, so falling
		// through with in.Choices empty is not itself a legal answer) would
		// reach for an activation and reintroduce I-1(b) through the
		// fallback path -- exactly what fix round 1 missed, because its own
		// synthetic regression decision carried a play_land option the loop
		// above matches unconditionally, a shape the live engine never
		// offers outside sorcery speed. Pass is offered on every priority
		// decision the engine emits; if it is somehow absent, this falls
		// through to the shared last resort below, same as any other kind.
		// M2d-3: the "concede" option sits directly after "pass" in the
		// option list, so this explicit scan is also what keeps the bot from
		// ever conceding -- it returns pass before clamp, or any
		// position-based fallback, can reach the new final option.
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
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

	case decision.KChoose:
		if len(d.Options) == 0 {
			break
		}
		switch d.Options[0].Kind {
		case "x":
			in.Choices = []int{d.Options[len(d.Options)-1].Index} // the most it can pay for
		case "exile", "sacrifice":
			for i := 0; i < len(d.Options) && i < d.Max; i++ {
				in.Choices = append(in.Choices, d.Options[i].Index)
			}
		default: // yes/no (yes is first), name, type, number: the first offer
			in.Choices = []int{d.Options[0].Index}
		}
		return clamp(d, in)

	case decision.KMulligan:
		// The London round, two shapes on one kind (rules/mulligan.go).
		// Bottoming: every option is a "bottom"; take the d.Min lowest-indexed
		// cards (the seat bottoms its oldest-held cards), no rng.
		if len(d.Options) > 0 && d.Options[0].Kind == "bottom" {
			for j := 0; j < len(d.Options) && j < d.Min; j++ {
				in.Choices = append(in.Choices, d.Options[j].Index)
			}
			return clamp(d, in)
		}
		// Keep/mulligan: mulligan with probability 1/3 when one is offered
		// (consuming the bot rng only where a real choice exists, the
		// determinism mirror of KTriggerOptional), else keep.
		if len(d.Options) > 1 {
			for _, o := range d.Options {
				if o.Kind == "mulligan" && r.IntN(3) == 0 {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index} // keep
			return clamp(d, in)
		}

	case decision.KModes:
		// The mid-resolution modal pick (M2d-2): choose the first Min options
		// in order — the recorded mirror of the engine-side first-mode
		// stand-in, so bot-vs-bot behaviour is largely unchanged, and the
		// answer stays seed-deterministic. This also answers an UnlessCost$
		// may-pay, shaped as the same KModes kind: option 0 is "Pay … — make
		// a copy", so the bot always offers to pay and the engine declines
		// for it only when the payer's pool cannot cover the cost. No rng is
		// consumed: the first modes are a fixed policy, not a coin.
		for j := 0; j < len(d.Options) && j < d.Min; j++ {
			in.Choices = append(in.Choices, d.Options[j].Index)
		}
		return clamp(d, in)
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
//
// Ruling T25-g (fix round 2): the top-up prefers an unused "pass" option
// over anything else. The KPriority branch above already returns "pass"
// explicitly before clamp ever runs, so this is defense in depth for
// whatever reaches here anyway (an absent "pass", or a future caller) --
// without it, a blind first-unused-index top-up would reach for whatever
// legalActions (rules/legal.go) happens to list first, which is every
// "activate" option, before "pass". That is precisely how fix round 1's own
// clamp reintroduced I-1(b): a Min:1 priority decision falling through with
// nothing chosen got topped up into an activation instead of a pass.
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
			if o.Kind == "pass" && !have[o.Index] {
				have[o.Index] = true
				in.Choices = append(in.Choices, o.Index)
			}
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
