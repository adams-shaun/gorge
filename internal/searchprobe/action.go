package searchprobe

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Action is a comparable semantic identity. Labels identify scalar choices,
// including modes that highlight their source. Object selections instead use
// stable observer-local identities, without cosmetic target labels.
type Action struct {
	Decision                      decision.Kind
	Source                        uint32
	Kind                          string
	Obj, Attacker                 uint32
	Player                        state.PlayerID
	Ability, AltCostIndex, Amount int
	Mode, SVar, Value             string
}

type ObservedOption struct {
	Action   Action
	Group    int
	Required bool
}
type ObservedDecision struct {
	Player      state.PlayerID
	Kind        decision.Kind
	Min, Max    int
	Source      uint32
	Options     []ObservedOption
	EffectAPI   string
	DamageKnown bool
	Damage      int
}

func (c *Collector) Actions(d *decision.Decision, in decision.Intent) ([]Action, error) {
	if d == nil || d.Player != c.actor {
		return nil, fmt.Errorf("may only capture the acting seat's own answer")
	}
	if err := d.Validate(in); err != nil {
		return nil, err
	}
	out := make([]Action, 0, len(in.Choices))
	for _, i := range in.Choices {
		a, err := c.action(d, d.Options[i])
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// Match must use the target world's collector after Capture has validated its
// observed prefix. Never reuse the source world's raw-ID dictionary.
func (c *Collector) Match(d *decision.Decision, actions []Action) (decision.Intent, error) {
	if d == nil || d.Player != c.actor {
		return decision.Intent{}, fmt.Errorf("may only match the acting seat's own answer")
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	for _, want := range actions {
		found := -1
		for i, o := range d.Options {
			got, err := c.action(d, o)
			if err != nil {
				return decision.Intent{}, err
			}
			if got == want {
				if found >= 0 {
					return decision.Intent{}, fmt.Errorf("ambiguous semantic action %+v", want)
				}
				found = i
			}
		}
		if found < 0 {
			return decision.Intent{}, fmt.Errorf("missing semantic action %+v", want)
		}
		in.Choices = append(in.Choices, found)
	}
	if err := d.Validate(in); err != nil {
		return decision.Intent{}, err
	}
	return in, nil
}

func (c *Collector) action(d *decision.Decision, o decision.Option) (Action, error) {
	a := Action{Decision: d.Kind, Source: c.ref(d.Source), Kind: o.Kind, Obj: c.ref(o.Obj), Attacker: c.ref(o.Attacker), Player: o.Player, Ability: o.Ability, AltCostIndex: o.AltCostIndex, Amount: o.Amount, Mode: o.Mode, SVar: o.SVar}
	if d.Source != 0 && a.Source == 0 || o.Obj != 0 && a.Obj == 0 || o.Attacker != 0 && a.Attacker == 0 {
		return Action{}, fmt.Errorf("action references an unobserved object")
	}
	// A mode's Obj highlights its source; it is not the selected value.
	// Pay/decline and Charm modes share that Obj and differ only in their
	// wire-visible label. Object-target labels remain cosmetic and omitted.
	if o.Obj == 0 || o.Kind == "mode" {
		a.Value = o.Label
	}
	return a, nil
}

func (c *Collector) observeDecision(d *decision.Decision) (*ObservedDecision, error) {
	if d == nil {
		return nil, nil
	}
	if d.Player != c.actor {
		return nil, fmt.Errorf("opponent private decision entered observation")
	}
	out := &ObservedDecision{Player: d.Player, Kind: d.Kind, Min: d.Min, Max: d.Max, Source: c.ref(d.Source)}
	groups := make(map[string]int)
	for _, o := range d.Options {
		a, err := c.action(d, o)
		if err != nil {
			return nil, err
		}
		group := 0
		if o.Group != "" {
			group = groups[o.Group]
			if group == 0 {
				group = len(groups) + 1
				groups[o.Group] = group
			}
		}
		out.Options = append(out.Options, ObservedOption{Action: a, Group: group, Required: o.Required})
	}
	if d.TargetEffect != nil {
		out.EffectAPI = d.TargetEffect.API
		if d.TargetEffect.Damage != nil && d.TargetEffect.Damage.Amount != nil {
			out.DamageKnown = true
			out.Damage = *d.TargetEffect.Damage.Amount
		}
	}
	return out, nil
}
