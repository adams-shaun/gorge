package oracle

import (
	"context"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
)

// PickCtx is what a Policy sees at one canonical decision.
type PickCtx struct {
	D   *CDecision // the leader's canonical decision (options sorted)
	G   *GorgeSide // the gorge side when it leads (nil when a kernel leads)
	Seq int        // canonical decision number in the game
}

// Policy picks one of a canonical decision's options. It must be a pure
// function of its own state and the context: a run is replayed by its
// action list, never by re-running the policy.
type Policy interface {
	Pick(c *PickCtx) Canon
}

// PolicyFunc is an external policy.
type PolicyFunc func(c *PickCtx) Canon

func (f PolicyFunc) Pick(c *PickCtx) Canon { return f(c) }

// AffordableTargetPolicy wraps p so a cast-time target ask whose price reads
// the target count (CDecision.TargetLeft from gorge's
// Decision.AffordableTargets) is closed with Done once the affordable picks
// are spent. Firing one more target only reverses the cast (CR 733.1): the
// caster cannot pay the final cost, so gorge's abortCast and the kernel's
// payment window both leave their taps in place, and the two engines choose
// the window's last source colour differently, so the floating pool diverges.
// gorge's botpolicy stays within the same count; a policy that does not is
// proposing a play the engine will undo. kfuzz's random policy wraps itself
// in this; a test that deliberately drives a CR 733.1 reversal keeps the raw
// policy.
func AffordableTargetPolicy(p Policy) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		if done, ok := affordableDone(c.D); ok {
			return done
		}
		return p.Pick(c)
	})
}

// affordableDone returns the Done canon when an unattended policy has spent
// a per-target-priced cast ask's affordable target count
// (CDecision.TargetLeft reaches zero).
func affordableDone(d *CDecision) (Canon, bool) {
	if d == nil || !d.TargetLimited || d.TargetLeft > 0 {
		return 0, false
	}
	for _, o := range d.Options {
		if o.Kind() == ActDone {
			return o, true
		}
	}
	return 0, false
}

// RandomPolicy picks uniformly among the options from its own PCG stream
// (never gorge's or the kernel's RNG).
func RandomPolicy(seed uint64) Policy {
	r := rand.New(rand.NewPCG(seed, 0x6b65726e656c)) // "kernel"
	return PolicyFunc(func(c *PickCtx) Canon { return c.D.Options[r.IntN(len(c.D.Options))] })
}

// FirstPolicy always picks the smallest action (Pass at priority: the game
// still advances). The shrinker's tail policy.
func FirstPolicy() Policy {
	return PolicyFunc(func(c *PickCtx) Canon { return c.D.Options[0] })
}

// HeuristicPolicy plays gorge's heuristic bot (auto-pay mana) through the
// canonical surface: at each new gorge decision it asks the bot, translates
// the bot's intent into canonical actions (a planned cast, an offered play,
// the picks of a multi-pick answer then Done) and serves them; an answer
// with no canonical form (a manual mana tap) falls back to the seeded
// random policy.
func HeuristicPolicy(seed uint64) Policy {
	h := &heurPolicy{bots: botsFor(heuristic.NewBot, seed),
		rnd: RandomPolicy(seed)}
	return h
}

// ExplorePolicy is HeuristicPolicy piloted by gorge's coverage-exploration
// bot (heuristic.NewExploreBot, auto-pay mana) on both seats, exactly as
// gorge's cmd/cardfuzz seats it: it tries every card's abilities, so a
// lockstep run under it compares more of each card's surface. Coverage
// (CoverageOf) reports which cards and abilities a run exercised.
func ExplorePolicy(seed uint64) Policy {
	return &heurPolicy{bots: botsFor(heuristic.NewExploreBot, seed),
		rnd: RandomPolicy(seed)}
}

// PolicyFallbacks is a bot policy's (HeuristicPolicy, ExplorePolicy) count
// of picks its bot's answer could not supply; 0 for any other policy.
func PolicyFallbacks(p Policy) int {
	if h, ok := p.(*heurPolicy); ok {
		return h.Fallbacks
	}
	return 0
}

type heurPolicy struct {
	bots  [MaxSeats]*heuristic.Bot
	rnd   Policy
	seq   uint64
	queue []Canon
	brd   botpolicy.Board
	// Fallbacks counts picks the bot's answer could not supply.
	Fallbacks int
}

func (h *heurPolicy) Pick(c *PickCtx) Canon {
	if c.G == nil {
		panic("oracle: HeuristicPolicy needs gorge to lead (PickCtx.G is nil in a kernel-led lockstep)")
	}
	d := c.G.Pending()
	if d != nil && d.Seq != h.seq {
		h.seq = d.Seq
		h.queue = h.translate(c, d)
	}
	for len(h.queue) > 0 {
		a := h.queue[0]
		h.queue = h.queue[1:]
		for _, o := range c.D.Options {
			if o == a {
				return a
			}
		}
	}
	h.Fallbacks++
	return h.rnd.Pick(c)
}

func (h *heurPolicy) translate(c *PickCtx, d *decision.Decision) []Canon {
	e := c.G.E
	if int(d.Player) >= len(h.bots) {
		return nil
	}
	e.EnsurePaymentActions()
	brd := botpolicy.BoardFromGameInto(e.G, e, d.Player, &h.brd)
	in, err := h.bots[d.Player].DecideBoard(context.Background(), brd, *d)
	if err != nil {
		return nil
	}
	if in.Payment != nil {
		for _, pa := range d.PaymentActions {
			if pa.ID == in.Payment.ActionID {
				mc, _ := castModeCode(pa.Cast.CastMode())
				return []Canon{Pack(ActCast, c.G.InstRef(pa.Cast.Object), RefNone, mc)}
			}
		}
		return nil
	}
	if d.Kind == decision.KPriority {
		if len(in.Choices) != 1 {
			return nil
		}
		o := optByIndex(d, in.Choices[0])
		switch o.Kind {
		case "pass":
			return []Canon{Pack(ActPass, RefNone, RefNone, 0)}
		case "play_land":
			return []Canon{Pack(ActPlayLand, c.G.InstRef(o.Obj), RefNone, landFaceCode(o.Mode))}
		case "cast":
			return []Canon{castOptCanon(c.G.InstRef(o.Obj), o.Mode, beamAlt(o.Mode, o.AltCostIndex, o.Cost))}
		case "ability":
			return []Canon{Pack(ActActivate, c.G.InstRef(o.Obj), RefNone, uint8(o.Ability))}
		}
		return nil
	}
	fr := c.G.fr
	if fr == nil || fr.seq != d.Seq {
		return nil
	}
	var out []Canon
	for _, ch := range in.Choices {
		for i := range fr.d.Options {
			if fr.d.Options[i].Index == ch && fr.valid[i] {
				out = append(out, fr.acts[i])
			}
		}
	}
	if fr.multi && len(out) < fr.d.Max {
		out = append(out, Pack(ActDone, RefNone, RefNone, 0))
	}
	return out
}

// botsFor is one bot a seat, seat s's seeded seed^(s+1): seats 0 and 1 are
// the two-seat game's bots, unchanged.
func botsFor(mk func(uint64) *heuristic.Bot, seed uint64) [MaxSeats]*heuristic.Bot {
	var b [MaxSeats]*heuristic.Bot
	for s := range b {
		b[s] = mk(seed ^ uint64(s+1)).EnableAutoPayMana()
	}
	return b
}
