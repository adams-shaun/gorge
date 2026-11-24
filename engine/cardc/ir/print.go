package ir

import (
	"strconv"
	"strings"
)

// Canonical printing. Every node prints as one line of S-expression, and two
// nodes print the same text iff Equal holds (Src is never printed; a nil slice
// and an empty one print alike).
//
// The scheme, the same for every type:
//
//		(TYPE [KIND] positional... :key value...)
//
//	  - TYPE is the lower-kebab type name. KIND, where the type has a kind, is
//	    the kind's name, always printed (so "none" prints).
//	  - Positional operands follow KIND in a fixed grammar, each distinguishable
//	    by shape (integer, bare word, quoted string, parenthesised node), in
//	    this order and only when non-zero:
//	    ref:    Slot (also when Kind is RefTarget, even at 0), then Of
//	    filter: Args, then Pred (bare word), then Word (quoted)
//	    value:  N (also when Kind is VConst, even at 0), then Args
//	    cond:   Args
//	    mana:   N
//	    compare: V
//	  - Every other field follows as a keyword (:lower-kebab-field-name), in
//	    declaration order, only when non-zero. A bool prints as the bare
//	    keyword. A slice prints [a b c]. A non-nil pointer prints even when its
//	    target is the zero value, so nil and non-nil differ.
//	  - Strings use strconv.Quote. Enums print their name. A ColorSet prints
//	    as its WUBRG letters, a ZoneSet as comma-joined zone names.
//	  - A nil node where a node is expected (a nil element of a slice) prints as
//	    the bare word nil.
//
// Appending an enum const changes nothing here: enums print through String.
type printer struct{ strings.Builder }

func (p *printer) open(head string) { p.WriteByte('('); p.WriteString(head) }
func (p *printer) end()             { p.WriteByte(')') }
func (p *printer) atom(s string)    { p.WriteByte(' '); p.WriteString(s) }
func (p *printer) key(k string)     { p.atom(":" + k) }

func (p *printer) kstr(k, s string) {
	if s != "" {
		p.key(k)
		p.atom(strconv.Quote(s))
	}
}

func (p *printer) kbare(k, s string) {
	if s != "" {
		p.key(k)
		p.atom(s)
	}
}

func (p *printer) kint(k string, n int32) {
	if n != 0 {
		p.key(k)
		p.atom(strconv.FormatInt(int64(n), 10))
	}
}

func (p *printer) kflag(k string, b bool) {
	if b {
		p.key(k)
	}
}

// kenum prints a non-zero enum by name.
func (p *printer) kenum(k string, nonzero bool, name string) {
	if nonzero {
		p.kbare(k, name)
	}
}

// pos prints a positional node operand (nil prints as nil).
func pos[T any](p *printer, n *T, f func(*printer, *T)) {
	if n == nil {
		p.atom("nil")
		return
	}
	p.WriteByte(' ')
	f(p, n)
}

// posOpt prints a positional node operand only when non-nil.
func posOpt[T any](p *printer, n *T, f func(*printer, *T)) {
	if n != nil {
		pos(p, n, f)
	}
}

// posList prints every element positionally.
func posList[T any](p *printer, xs []*T, f func(*printer, *T)) {
	for _, x := range xs {
		pos(p, x, f)
	}
}

// knode prints a keyword with a node operand when the pointer is non-nil.
func knode[T any](p *printer, k string, n *T, f func(*printer, *T)) {
	if n != nil {
		p.key(k)
		pos(p, n, f)
	}
}

// klist prints a keyword with a bracketed list when the slice is non-empty.
func klist[T any](p *printer, k string, xs []T, f func(*printer, T)) {
	if len(xs) == 0 {
		return
	}
	p.key(k)
	p.atom("[")
	for i, x := range xs {
		if i > 0 {
			p.WriteByte(' ')
		}
		f(p, x)
	}
	p.WriteByte(']')
}

// knodes prints a keyword with a list of nodes (nil elements print as nil).
func knodes[T any](p *printer, k string, xs []*T, f func(*printer, *T)) {
	klist(p, k, xs, func(p *printer, x *T) {
		if x == nil {
			p.WriteString("nil")
			return
		}
		f(p, x)
	})
}

func prQuoted(p *printer, s string) { p.WriteString(strconv.Quote(s)) }

func prMana(p *printer, m ManaSym) {
	p.open("mana")
	p.atom(m.Kind.String())
	if m.N != 0 {
		p.atom(strconv.FormatInt(int64(m.N), 10))
	}
	p.kbare("colors", m.Colors.String())
	p.end()
}

func prRefusal(p *printer, r Refusal) {
	p.open("refusal")
	p.kstr("key", r.Key)
	p.kstr("path", r.Path)
	p.end()
}

func prCard(p *printer, n *Card) {
	p.open("card")
	p.kstr("name", n.Name)
	p.kenum("layout", n.Layout != 0, n.Layout.String())
	knodes(p, "faces", n.Faces, prFace)
	p.end()
}

func prFace(p *printer, n *Face) {
	p.open("face")
	p.kstr("name", n.Name)
	klist(p, "mana-cost", n.ManaCost, prMana)
	p.kflag("has-mana-cost", n.HasManaCost)
	p.kbare("color-indicator", n.ColorIndicator.String())
	klist(p, "supertypes", n.Supertypes, prQuoted)
	klist(p, "types", n.Types, prQuoted)
	klist(p, "subtypes", n.Subtypes, prQuoted)
	knode(p, "power", n.Power, prPT)
	knode(p, "toughness", n.Toughness, prPT)
	if n.Loyalty != nil {
		p.key("loyalty")
		p.atom(strconv.FormatInt(int64(*n.Loyalty), 10))
	}
	if n.Defense != nil {
		p.key("defense")
		p.atom(strconv.FormatInt(int64(*n.Defense), 10))
	}
	knodes(p, "abilities", n.Abilities, prAbility)
	klist(p, "refusals", n.Refusals, prRefusal)
	p.end()
}

func prPT(p *printer, n *PT) {
	p.open("pt")
	p.kint("n", n.N)
	p.kflag("star", n.Star)
	p.end()
}

func prCompare(p *printer, n *Compare) {
	p.open("compare")
	p.atom(n.Op.String())
	posOpt(p, n.V, prValue)
	p.end()
}

func prRef(p *printer, n *Ref) {
	p.open("ref")
	p.atom(n.Kind.String())
	if n.Kind == RefTarget || n.Slot != 0 {
		p.atom(strconv.Itoa(int(n.Slot)))
	}
	posOpt(p, n.Of, prRef)
	knode(p, "filter", n.Filter, prFilter)
	knode(p, "n", n.N, prValue)
	p.kenum("receipt", n.Receipt != 0, n.Receipt.String())
	knodes(p, "union", n.Union, prRef)
	p.end()
}

func prTarget(p *printer, n *Target) {
	p.open("target")
	p.kint("min", n.Min)
	p.kint("max", n.Max)
	knode(p, "max-v", n.MaxV, prValue)
	knode(p, "filter", n.Filter, prFilter)
	p.kflag("distinct", n.Distinct)
	p.end()
}

func prFilter(p *printer, n *Filter) {
	p.open("filter")
	p.atom(n.Op.String())
	posList(p, n.Args, prFilter)
	if n.Pred != PNone {
		p.atom(n.Pred.String())
	}
	if n.Word != "" {
		p.atom(strconv.Quote(n.Word))
	}
	p.kenum("subject", n.Subject != 0, n.Subject.String())
	p.kbare("zones", n.Zones.String())
	p.kbare("colors", n.Colors.String())
	knode(p, "ref", n.Ref, prRef)
	p.kenum("cmp", n.Cmp != 0, n.Cmp.String())
	knode(p, "value", n.Value, prValue)
	p.end()
}

func prValue(p *printer, n *Value) {
	p.open("value")
	p.atom(n.Kind.String())
	if n.Kind == VConst || n.N != 0 {
		p.atom(strconv.FormatInt(int64(n.N), 10))
	}
	posList(p, n.Args, prValue)
	knode(p, "filter", n.Filter, prFilter)
	knode(p, "ref", n.Ref, prRef)
	knode(p, "ref2", n.Ref2, prRef)
	p.kstr("word", n.Word)
	p.kbare("colors", n.Colors.String())
	p.kenum("receipt", n.Receipt != 0, n.Receipt.String())
	p.end()
}

func prCond(p *printer, n *Cond) {
	p.open("cond")
	p.atom(n.Kind.String())
	posList(p, n.Args, prCond)
	knode(p, "l", n.L, prValue)
	knode(p, "r", n.R, prValue)
	p.kenum("cmp", n.Cmp != 0, n.Cmp.String())
	knode(p, "filter", n.Filter, prFilter)
	knode(p, "ref", n.Ref, prRef)
	p.kstr("word", n.Word)
	p.kenum("receipt", n.Receipt != 0, n.Receipt.String())
	p.end()
}

func prAbility(p *printer, n *Ability) {
	p.open("ability")
	p.atom(n.Kind.String())
	knode(p, "cost", n.Cost, prCost)
	knode(p, "trigger", n.Trigger, prTrigger)
	knode(p, "cond", n.Cond, prCond)
	knode(p, "body", n.Body, prProgram)
	knode(p, "keyword", n.Keyword, prKeyword)
	p.kbare("zones", n.Zones.String())
	p.kflag("sorcery-only", n.SorceryOnly)
	p.kflag("instant-only", n.InstantOnly)
	p.kflag("once", n.Once)
	p.end()
}

func prKeyword(p *printer, n *Keyword) {
	p.open("keyword")
	p.kstr("name", n.Name)
	knode(p, "n", n.N, prValue)
	knode(p, "cost", n.Cost, prCost)
	knode(p, "filter", n.Filter, prFilter)
	p.end()
}

func prTrigger(p *printer, n *Trigger) {
	p.open("trigger")
	p.atom(n.Event.String())
	knode(p, "subject", n.Subject, prFilter)
	knode(p, "ref", n.Ref, prRef)
	p.kbare("zones", n.Zones.String())
	p.kflag("once-per-turn", n.OncePerTurn)
	knode(p, "object", n.Object, prFilter)
	knode(p, "cmp", n.Cmp, prCompare)
	p.kstr("word", n.Word)
	knode(p, "cond", n.Cond, prCond)
	knodes(p, "args", n.Args, prTrigger)
	p.end()
}

func prProgram(p *printer, n *Program) {
	p.open("program")
	knodes(p, "steps", n.Steps, prStep)
	knodes(p, "targets", n.Targets, prTarget)
	p.end()
}

func prStep(p *printer, n *Step) {
	p.open("step")
	p.atom(n.Verb.String())
	p.kflag("optional", n.Optional)
	knode(p, "who", n.Who, prRef)
	knode(p, "obj", n.Obj, prRef)
	knode(p, "n", n.N, prValue)
	knode(p, "cond", n.Cond, prCond)
	knode(p, "then", n.Then, prProgram)
	knode(p, "else", n.Else, prProgram)
	p.kenum("dur", n.Dur != 0, n.Dur.String())
	p.kenum("from", n.From != 0, n.From.String())
	p.kenum("to", n.To != 0, n.To.String())
	p.kstr("word", n.Word)
	p.kbare("colors", n.Colors.String())
	knode(p, "source", n.Source, prRef)
	knode(p, "other", n.Other, prRef)
	knode(p, "filter", n.Filter, prFilter)
	knode(p, "cost", n.Cost, prCost)
	klist(p, "mana", n.Mana, prMana)
	knodes(p, "modes", n.Modes, prProgram)
	knodes(p, "effects", n.Effects, prEffect)
	knodes(p, "abilities", n.Abilities, prAbility)
	knode(p, "token", n.Token, prFace)
	knode(p, "enter", n.Enter, prEnter)
	p.end()
}

func prEffect(p *printer, n *Effect) {
	p.open("effect")
	p.atom(n.Kind.String())
	knode(p, "p", n.P, prValue)
	knode(p, "t", n.T, prValue)
	knode(p, "n", n.N, prValue)
	klist(p, "words", n.Words, prQuoted)
	p.kbare("colors", n.Colors.String())
	knodes(p, "abilities", n.Abilities, prAbility)
	knode(p, "filter", n.Filter, prFilter)
	knode(p, "ref", n.Ref, prRef)
	knode(p, "cost", n.Cost, prCost)
	knode(p, "cond", n.Cond, prCond)
	p.end()
}

func prEnter(p *printer, n *Enter) {
	p.open("enter")
	p.kflag("tapped", n.Tapped)
	p.kflag("under-owner", n.UnderOwner)
	knode(p, "controller", n.Controller, prRef)
	p.kstr("counter", n.Counter)
	knode(p, "n", n.N, prValue)
	knodes(p, "effects", n.Effects, prEffect)
	p.end()
}

func prCost(p *printer, n *Cost) {
	p.open("cost")
	klist(p, "mana", n.Mana, prMana)
	p.kflag("has-mana", n.HasMana)
	knodes(p, "parts", n.Parts, prCostPart)
	p.end()
}

func prCostPart(p *printer, n *CostPart) {
	p.open("cost-part")
	p.atom(n.Kind.String())
	knode(p, "obj", n.Obj, prRef)
	knode(p, "n", n.N, prValue)
	p.kstr("word", n.Word)
	p.end()
}

func render[T any](n *T, f func(*printer, *T)) string {
	if n == nil {
		return "nil"
	}
	var p printer
	f(&p, n)
	return p.String()
}

// String returns the canonical S-expression of the node (see print.go).
func (n *Card) String() string     { return render(n, prCard) }
func (n *Face) String() string     { return render(n, prFace) }
func (n *PT) String() string       { return render(n, prPT) }
func (n *Compare) String() string  { return render(n, prCompare) }
func (n *Ref) String() string      { return render(n, prRef) }
func (n *Target) String() string   { return render(n, prTarget) }
func (n *Filter) String() string   { return render(n, prFilter) }
func (n *Value) String() string    { return render(n, prValue) }
func (n *Cond) String() string     { return render(n, prCond) }
func (n *Ability) String() string  { return render(n, prAbility) }
func (n *Keyword) String() string  { return render(n, prKeyword) }
func (n *Trigger) String() string  { return render(n, prTrigger) }
func (n *Program) String() string  { return render(n, prProgram) }
func (n *Step) String() string     { return render(n, prStep) }
func (n *Effect) String() string   { return render(n, prEffect) }
func (n *Enter) String() string    { return render(n, prEnter) }
func (n *Cost) String() string     { return render(n, prCost) }
func (n *CostPart) String() string { return render(n, prCostPart) }

func (m ManaSym) String() string {
	var p printer
	prMana(&p, m)
	return p.String()
}

func (r Refusal) String() string {
	var p printer
	prRefusal(&p, r)
	return p.String()
}
