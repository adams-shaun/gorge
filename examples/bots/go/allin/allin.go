// Package allin is a minimal native gorge bot policy: the production
// heuristic bot, except that it attacks with every creature whose power is at
// least the configured minimum, every combat. It shows the three things every
// native policy does:
//
//  1. implement seat.Seat (Decide), answering from the seat's redacted View;
//  2. delegate the decisions it does not specialise in to the default bot;
//  3. register itself, with its options, from an init function.
//
// Link it into a binary with a blank import:
//
//	import _ "github.com/adams-shaun/gorge/examples/bots/go/allin"
package allin

import (
	"context"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// Name is the registry key: what -bot-policy, POST /api/games' bot_policy
// and -bot-opt allin.<key>=<value> use.
const Name = "allin"

func init() {
	bots.Register(bots.Entry{
		Info: bots.Info{
			Name:        Name,
			Label:       "All-in (example)",
			Description: "The heuristic bot, but it attacks with every creature of at least min-power power, every combat.",
			Tier:        bots.Experimental,
			Formats:     []string{"constructed", "commander"},
			Options: []bots.OptionSpec{{
				Key:     "min-power",
				Type:    "int",
				Default: "1",
				Help:    "attack only with creatures of at least this power",
			}},
		},
		New: New,
	})
}

// New is the factory. bots.New fills every declared option's default into
// o.Config first, so o.Int("min-power") is always set.
func New(o bots.Options) (seat.Seat, error) {
	b := heuristic.NewBot(o.Seed)
	if o.AutoPayMana {
		b.EnableAutoPayMana()
	}
	return &Seat{fallback: b, minPower: int32(o.Int("min-power"))}, nil
}

// Seat is the policy. It holds no engine and no game: everything it knows
// arrives in Decide's View and Decision.
type Seat struct {
	fallback *heuristic.Bot
	minPower int32
}

// WantsPaymentActions forwards the wrapped bot's answer, so on an auto-pay
// table the engine attaches mana-payment plans to the decisions this seat
// passes through (seat.PaymentPlanConsumer).
func (s *Seat) WantsPaymentActions() bool { return s.fallback.WantsPaymentActions() }

// Decide answers one decision.
func (s *Seat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if d.Kind != decision.KAttackers {
		return s.fallback.Decide(ctx, v, d)
	}
	power := map[state.ObjID]int32{}
	for _, p := range v.Players {
		for _, c := range p.Battlefield {
			power[c.ID] = c.Power
		}
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	attacking := map[state.ObjID]bool{} // one defender per creature
	groups := map[string]bool{}         // at most one option per Group
	for _, o := range d.Options {
		if attacking[o.Obj] || (o.Group != "" && groups[o.Group]) {
			continue
		}
		if !o.Required && power[o.Obj] < s.minPower {
			continue
		}
		in.Choices = append(in.Choices, o.Index)
		attacking[o.Obj] = true
		if o.Group != "" {
			groups[o.Group] = true
		}
	}
	// The engine is the judge: if a whole-declaration rule (an attack tax, a
	// "can't attack alone") makes this answer illegal, play the bot's.
	if d.Validate(in) != nil {
		return s.fallback.Decide(ctx, v, d)
	}
	return in, nil
}
