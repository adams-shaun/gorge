package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Program plumbing and the Actions containers: typed helpers over Ctx.lower,
// step splicing, and target frames.

// Program lowers x to an *ir.Program.
func (c *Ctx) Program(x any) (*ir.Program, error) { return lowerAs[ir.Program](c, x) }

// Cost lowers x to an *ir.Cost.
func (c *Ctx) Cost(x any) (*ir.Cost, error) { return lowerAs[ir.Cost](c, x) }

// Effect lowers x to an *ir.Effect.
func (c *Ctx) Effect(x any) (*ir.Effect, error) { return lowerAs[ir.Effect](c, x) }

// Trigger lowers x to an *ir.Trigger.
func (c *Ctx) Trigger(x any) (*ir.Trigger, error) { return lowerAs[ir.Trigger](c, x) }

// Enter lowers x to an *ir.Enter.
func (c *Ctx) Enter(x any) (*ir.Enter, error) { return lowerAs[ir.Enter](c, x) }

// Steps lowers an action. A lowering returns one *ir.Step, or an *ir.Program
// whose steps are spliced into the enclosing list in order.
func (c *Ctx) Steps(x any) ([]*ir.Step, error) {
	v, err := c.lower(x)
	if err != nil {
		return nil, err
	}
	switch s := v.(type) {
	case *ir.Step:
		return []*ir.Step{s}, nil
	case *ir.Program:
		if len(s.Targets) > 0 {
			return nil, fmt.Errorf("cardc: %s: cannot splice a program that declares targets", c.Src())
		}
		return s.Steps, nil
	}
	return nil, fmt.Errorf("%s lowers to %T, want a step or a program", qualified(x), v)
}

// StepList is the Program of every element's Steps, concatenated.
func (c *Ctx) StepList(xs []any) (*ir.Program, error) {
	p := &ir.Program{}
	for _, x := range xs {
		ss, err := c.Steps(x)
		if err != nil {
			return nil, err
		}
		p.Steps = append(p.Steps, ss...)
	}
	return p, nil
}

// Duration lowers an Expiration. Its lowerings return *ir.Duration.
func (c *Ctx) Duration(x any) (ir.Duration, error) {
	d, err := lowerAs[ir.Duration](c, x)
	if err != nil {
		return ir.DurNone, err
	}
	return *d, nil
}

// Effects lowers each element to an *ir.Effect.
func (c *Ctx) Effects(xs []any) ([]*ir.Effect, error) {
	var out []*ir.Effect
	for _, x := range xs {
		e, err := c.Effect(x)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// Enters merges a flag list into one *ir.Enter: bools OR together; a pointer
// or string field set by two flags is an error. An empty list, or one that
// sets nothing (EnterFlag.EntersNormally), gives nil.
func (c *Ctx) Enters(xs []any) (*ir.Enter, error) {
	out := &ir.Enter{}
	for _, x := range xs {
		e, err := c.Enter(x)
		if err != nil {
			return nil, err
		}
		out.Tapped = out.Tapped || e.Tapped
		out.UnderOwner = out.UnderOwner || e.UnderOwner
		if e.Controller != nil {
			if out.Controller != nil {
				return nil, fmt.Errorf("%s: conflicting enter flags", qualified(x))
			}
			out.Controller = e.Controller
		}
		if e.Counter != "" || e.N != nil {
			if out.Counter != "" || out.N != nil {
				return nil, fmt.Errorf("%s: conflicting enter flags", qualified(x))
			}
			out.Counter, out.N = e.Counter, e.N
		}
		out.Effects = append(out.Effects, e.Effects...)
	}
	if !out.Tapped && !out.UnderOwner && out.Controller == nil && out.Counter == "" && out.N == nil && len(out.Effects) == 0 {
		return nil, nil
	}
	out.Src = c.Src()
	return out, nil
}

// withTargetFrame runs f with an empty target-declaration list and restores
// the caller's list afterwards, so f's declarations get slots from 0 and the
// caller's Ref_ variants resolve after f exactly as before it.
func (c *Ctx) withTargetFrame(f func() error) error {
	saved := c.targets
	c.targets = nil
	err := f()
	c.targets = saved
	return err
}

// NestedAbilities lowers each rule in its own target frame. Every lowering
// that holds a Rule list calls it: granted abilities, emblems, token rules.
func (c *Ctx) NestedAbilities(rules []any) ([]*ir.Ability, error) {
	var out []*ir.Ability
	for _, r := range rules {
		var ab *ir.Ability
		err := c.withTargetFrame(func() (err error) {
			ab, err = c.Ability(r)
			return err
		})
		if err != nil {
			return nil, err
		}
		out = append(out, ab)
	}
	return out, nil
}

// NestedBody lowers a delayed or reflexive trigger's Actions in its own
// target frame.
func (c *Ctx) NestedBody(x any) (*ir.Program, error) {
	var p *ir.Program
	err := c.withTargetFrame(func() (err error) {
		p, err = c.Program(x)
		return err
	})
	return p, err
}

// withZone reads a Cards filter in one zone.
func withZone(f *ir.Filter, z ir.Zone) *ir.Filter {
	return &ir.Filter{Subject: ir.SubjCard, Zones: ir.MakeZoneSet(z), Op: ir.FAll, Args: []*ir.Filter{f}}
}

// anyCardIn is any card in zone z.
func anyCardIn(z ir.Zone) *ir.Filter {
	return &ir.Filter{Subject: ir.SubjCard, Zones: ir.MakeZoneSet(z), Op: ir.FPred, Pred: ir.PAny}
}

func init() {
	register("Actions.ActionList", func(c *Ctx, n node) (any, error) {
		return c.StepList(list(rawArgs(n)))
	})
	register("Actions.Targeted", func(c *Ctx, n node) (any, error) {
		var out *ir.Program
		err := c.withTargetFrame(func() error {
			var targets []*ir.Target
			for _, t := range list(argN(n, 0)) {
				tg, err := c.Target(t)
				if err != nil {
					return err
				}
				targets = append(targets, tg)
			}
			body, err := c.Program(argN(n, 1))
			if err != nil {
				return err
			}
			if len(body.Targets) > 0 {
				return fmt.Errorf("Actions.Targeted: nested target list")
			}
			out = &ir.Program{Steps: body.Steps, Targets: targets}
			return nil
		})
		return out, err
	})
	register("Actions.Modal_ChooseOne", func(c *Ctx, n node) (any, error) {
		st := &ir.Step{Verb: ir.VerbChooseModes, N: refConst1()}
		for _, m := range list(rawArgs(n)) {
			p, err := c.Program(m)
			if err != nil {
				return nil, err
			}
			st.Modes = append(st.Modes, p)
		}
		return &ir.Program{Steps: []*ir.Step{st}}, nil
	})
}
