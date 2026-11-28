package ir

import "slices"

// Equal reports whether a and b, node pointers such as *Card or *Ref, are
// structurally equal, ignoring every Src field. Values of different dynamic
// types are unequal, as is anything that is not an IR node pointer; two nil
// pointers of one type are equal. A nil slice equals an empty one (they encode the same); a nil pointer
// differs from a non-nil one.
//
// One function per type, written by hand: a new field is one new line here,
// and appending an enum const needs no edit.
func Equal(a, b any) bool {
	switch x := a.(type) {
	case *Card:
		y, ok := b.(*Card)
		return ok && eqPtr(x, y, eqCard)
	case *Face:
		y, ok := b.(*Face)
		return ok && eqPtr(x, y, eqFace)
	case *PT:
		y, ok := b.(*PT)
		return ok && eqPtr(x, y, eqPT)
	case *Refusal:
		y, ok := b.(*Refusal)
		return ok && eqPtr(x, y, eqRefusal)
	case *ManaSym:
		y, ok := b.(*ManaSym)
		return ok && eqPtr(x, y, eqManaSym)
	case *Compare:
		y, ok := b.(*Compare)
		return ok && eqPtr(x, y, eqCompare)
	case *Ref:
		y, ok := b.(*Ref)
		return ok && eqPtr(x, y, eqRef)
	case *Target:
		y, ok := b.(*Target)
		return ok && eqPtr(x, y, eqTarget)
	case *Filter:
		y, ok := b.(*Filter)
		return ok && eqPtr(x, y, eqFilter)
	case *Value:
		y, ok := b.(*Value)
		return ok && eqPtr(x, y, eqValue)
	case *Cond:
		y, ok := b.(*Cond)
		return ok && eqPtr(x, y, eqCond)
	case *Ability:
		y, ok := b.(*Ability)
		return ok && eqPtr(x, y, eqAbility)
	case *Keyword:
		y, ok := b.(*Keyword)
		return ok && eqPtr(x, y, eqKeyword)
	case *Trigger:
		y, ok := b.(*Trigger)
		return ok && eqPtr(x, y, eqTrigger)
	case *Program:
		y, ok := b.(*Program)
		return ok && eqPtr(x, y, eqProgram)
	case *Step:
		y, ok := b.(*Step)
		return ok && eqPtr(x, y, eqStep)
	case *Effect:
		y, ok := b.(*Effect)
		return ok && eqPtr(x, y, eqEffect)
	case *Enter:
		y, ok := b.(*Enter)
		return ok && eqPtr(x, y, eqEnter)
	case *Cost:
		y, ok := b.(*Cost)
		return ok && eqPtr(x, y, eqCost)
	case *CostPart:
		y, ok := b.(*CostPart)
		return ok && eqPtr(x, y, eqCostPart)
	}
	return false // not an IR node pointer
}

// eqPtr compares two pointers: both nil, or both non-nil and eq.
func eqPtr[T any](a, b *T, eq func(a, b *T) bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return eq(a, b)
}

// eqSlice compares slices element-wise; nil equals empty.
func eqSlice[T any](a, b []T, eq func(a, b T) bool) bool {
	return slices.EqualFunc(a, b, eq)
}

func eqPtrSlice[T any](a, b []*T, eq func(a, b *T) bool) bool {
	return slices.EqualFunc(a, b, func(x, y *T) bool { return eqPtr(x, y, eq) })
}

func eqInt32(a, b *int32) bool { return *a == *b }

func eqCard(a, b *Card) bool {
	return a.Name == b.Name && a.Layout == b.Layout && eqPtrSlice(a.Faces, b.Faces, eqFace)
}

func eqFace(a, b *Face) bool {
	return a.Name == b.Name &&
		eqSlice(a.ManaCost, b.ManaCost, func(x, y ManaSym) bool { return x == y }) &&
		a.HasManaCost == b.HasManaCost &&
		a.ColorIndicator == b.ColorIndicator &&
		slices.Equal(a.Supertypes, b.Supertypes) &&
		slices.Equal(a.Types, b.Types) &&
		slices.Equal(a.Subtypes, b.Subtypes) &&
		eqPtr(a.Power, b.Power, eqPT) &&
		eqPtr(a.Toughness, b.Toughness, eqPT) &&
		eqPtr(a.Loyalty, b.Loyalty, eqInt32) &&
		eqPtr(a.Defense, b.Defense, eqInt32) &&
		eqPtrSlice(a.Abilities, b.Abilities, eqAbility) &&
		slices.Equal(a.Refusals, b.Refusals)
}

func eqPT(a, b *PT) bool { return a.N == b.N && a.Star == b.Star }

func eqRefusal(a, b *Refusal) bool { return *a == *b }

func eqManaSym(a, b *ManaSym) bool { return *a == *b }

func eqCompare(a, b *Compare) bool {
	return a.Op == b.Op && eqPtr(a.V, b.V, eqValue)
}

func eqRef(a, b *Ref) bool {
	return a.Kind == b.Kind && a.Slot == b.Slot &&
		eqPtr(a.Of, b.Of, eqRef) &&
		eqPtr(a.Filter, b.Filter, eqFilter) &&
		eqPtr(a.N, b.N, eqValue) &&
		a.UpTo == b.UpTo &&
		a.Receipt == b.Receipt &&
		eqPtrSlice(a.Union, b.Union, eqRef)
}

func eqTarget(a, b *Target) bool {
	return a.Min == b.Min && a.Max == b.Max &&
		eqPtr(a.MaxV, b.MaxV, eqValue) &&
		eqPtr(a.Filter, b.Filter, eqFilter) &&
		a.Distinct == b.Distinct
}

func eqFilter(a, b *Filter) bool {
	return a.Subject == b.Subject && a.Zones == b.Zones && a.Op == b.Op &&
		eqPtrSlice(a.Args, b.Args, eqFilter) &&
		a.Pred == b.Pred && a.Word == b.Word && a.Colors == b.Colors &&
		eqPtr(a.Ref, b.Ref, eqRef) &&
		a.Cmp == b.Cmp &&
		eqPtr(a.Value, b.Value, eqValue)
}

func eqValue(a, b *Value) bool {
	return a.Kind == b.Kind && a.N == b.N &&
		eqPtrSlice(a.Args, b.Args, eqValue) &&
		eqPtr(a.Filter, b.Filter, eqFilter) &&
		eqPtr(a.Ref, b.Ref, eqRef) &&
		eqPtr(a.Ref2, b.Ref2, eqRef) &&
		a.Word == b.Word && a.Colors == b.Colors && a.Receipt == b.Receipt
}

func eqCond(a, b *Cond) bool {
	return a.Kind == b.Kind &&
		eqPtrSlice(a.Args, b.Args, eqCond) &&
		eqPtr(a.L, b.L, eqValue) && eqPtr(a.R, b.R, eqValue) &&
		a.Cmp == b.Cmp &&
		eqPtr(a.Filter, b.Filter, eqFilter) &&
		eqPtr(a.Ref, b.Ref, eqRef) &&
		a.Word == b.Word && a.Receipt == b.Receipt
}

func eqAbility(a, b *Ability) bool {
	return a.Kind == b.Kind &&
		eqPtr(a.Cost, b.Cost, eqCost) &&
		eqPtr(a.Trigger, b.Trigger, eqTrigger) &&
		eqPtr(a.Cond, b.Cond, eqCond) &&
		eqPtr(a.Body, b.Body, eqProgram) &&
		eqPtr(a.Keyword, b.Keyword, eqKeyword) &&
		a.Zones == b.Zones &&
		a.SorceryOnly == b.SorceryOnly && a.InstantOnly == b.InstantOnly && a.Once == b.Once
}

func eqKeyword(a, b *Keyword) bool {
	return a.Name == b.Name &&
		eqPtr(a.N, b.N, eqValue) &&
		eqPtr(a.Cost, b.Cost, eqCost) &&
		eqPtr(a.Filter, b.Filter, eqFilter)
}

func eqTrigger(a, b *Trigger) bool {
	return a.Event == b.Event &&
		eqPtr(a.Subject, b.Subject, eqFilter) &&
		eqPtr(a.Ref, b.Ref, eqRef) &&
		a.Zones == b.Zones && a.OncePerTurn == b.OncePerTurn &&
		eqPtr(a.Object, b.Object, eqFilter) &&
		eqPtr(a.Cmp, b.Cmp, eqCompare) &&
		a.Word == b.Word &&
		eqPtr(a.Cond, b.Cond, eqCond) &&
		eqPtrSlice(a.Args, b.Args, eqTrigger)
}

func eqProgram(a, b *Program) bool {
	return eqPtrSlice(a.Steps, b.Steps, eqStep) && eqPtrSlice(a.Targets, b.Targets, eqTarget)
}

func eqStep(a, b *Step) bool {
	return a.Verb == b.Verb && a.Optional == b.Optional &&
		eqPtr(a.Who, b.Who, eqRef) && eqPtr(a.Obj, b.Obj, eqRef) &&
		eqPtr(a.N, b.N, eqValue) && eqPtr(a.Cond, b.Cond, eqCond) &&
		eqPtr(a.Then, b.Then, eqProgram) && eqPtr(a.Else, b.Else, eqProgram) &&
		a.Dur == b.Dur && a.From == b.From && a.To == b.To &&
		a.Word == b.Word && a.Colors == b.Colors &&
		eqPtr(a.Source, b.Source, eqRef) && eqPtr(a.Other, b.Other, eqRef) &&
		eqPtr(a.Filter, b.Filter, eqFilter) && eqPtr(a.Cost, b.Cost, eqCost) &&
		slices.Equal(a.Mana, b.Mana) &&
		eqPtrSlice(a.Modes, b.Modes, eqProgram) &&
		eqPtrSlice(a.Effects, b.Effects, eqEffect) &&
		eqPtrSlice(a.Abilities, b.Abilities, eqAbility) &&
		eqPtr(a.Token, b.Token, eqFace) && eqPtr(a.Enter, b.Enter, eqEnter)
}

func eqEffect(a, b *Effect) bool {
	return a.Kind == b.Kind &&
		eqPtr(a.P, b.P, eqValue) && eqPtr(a.T, b.T, eqValue) && eqPtr(a.N, b.N, eqValue) &&
		slices.Equal(a.Words, b.Words) && a.Colors == b.Colors &&
		eqPtrSlice(a.Abilities, b.Abilities, eqAbility) &&
		eqPtr(a.Filter, b.Filter, eqFilter) && eqPtr(a.Ref, b.Ref, eqRef) &&
		eqPtr(a.Cost, b.Cost, eqCost) && eqPtr(a.Cond, b.Cond, eqCond)
}

func eqEnter(a, b *Enter) bool {
	return a.Tapped == b.Tapped && a.UnderOwner == b.UnderOwner &&
		eqPtr(a.Controller, b.Controller, eqRef) &&
		a.Counter == b.Counter && eqPtr(a.N, b.N, eqValue) &&
		eqPtrSlice(a.Effects, b.Effects, eqEffect)
}

func eqCost(a, b *Cost) bool {
	return slices.Equal(a.Mana, b.Mana) && a.HasMana == b.HasMana &&
		eqPtrSlice(a.Parts, b.Parts, eqCostPart)
}

func eqCostPart(a, b *CostPart) bool {
	return a.Kind == b.Kind && eqPtr(a.Obj, b.Obj, eqRef) &&
		eqPtr(a.N, b.N, eqValue) && a.Word == b.Word
}
