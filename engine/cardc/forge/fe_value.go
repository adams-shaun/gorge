package forge

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// SVarLookup returns the body of the card's SVar name.
type SVarLookup func(name string) (string, bool)

// maxSVarDepth is the longest SVar name chain the reader follows.
const maxSVarDepth = 4

// LowerValue reads a Forge numeric expression into an ir.Value, or refuses it
// naming the head or operator it cannot read. It reads nothing of the card but
// svars, which it follows from a name to its body, at most maxSVarDepth names
// deep.
//
// Forms, tried in order: an integer (optional sign); a signed form ("+X",
// "-X"); a name (X with no SVar named X is VX); Count$<head>[/<op>];
// Number$<n>[/<op>]; SVar$<name>[/<op>]; <Referent>$<Property>, refused as
// needs-reference until the references reader lands.
func LowerValue(text string, svars SVarLookup) (*ir.Value, *Refusal) {
	if svars == nil {
		svars = func(string) (string, bool) { return "", false }
	}
	rd := &valueReader{svars: svars}
	return rd.text(text)
}

// valueReader carries the lookup and the SVar names being followed.
type valueReader struct {
	svars SVarLookup
	path  []string
}

func (rd *valueReader) text(text string) (*ir.Value, *Refusal) {
	text = strings.TrimSpace(text)
	v, r := rd.form(text)
	if r != nil {
		return nil, r
	}
	stampSrc(v, text)
	return v, nil
}

func (rd *valueReader) form(text string) (*ir.Value, *Refusal) {
	if text == "" {
		return nil, refuse(RefuseEmpty, "")
	}
	if n, err := strconv.ParseInt(text, 10, 32); err == nil {
		return constV(int32(n)), nil
	}
	if rest, ok := strings.CutPrefix(text, "+"); ok {
		return rd.text(rest)
	}
	if rest, ok := strings.CutPrefix(text, "-"); ok {
		v, r := rd.text(rest)
		if r != nil {
			return nil, r
		}
		return binary(ir.VMul, constV(-1), v), nil
	}
	if isName(text) {
		return rd.name(text)
	}
	head, body, ok := strings.Cut(text, "$")
	if !ok || !isName(head) || strings.Contains(text, "|") || strings.HasPrefix(body, " ") {
		return nil, refuse(RefuseNotANumber, leadWord(text))
	}
	expr, op, hasOp := strings.Cut(body, "/")
	v, r := rd.dollar(head, expr)
	if r != nil || !hasOp {
		return v, r
	}
	return rd.op(v, op)
}

// dollar reads <head>$<expr>, the operator already cut off.
func (rd *valueReader) dollar(head, expr string) (*ir.Value, *Refusal) {
	switch head {
	case "Count":
		return rd.count(strings.TrimSpace(expr))
	case "Number":
		n, err := strconv.ParseInt(strings.TrimSpace(expr), 10, 32)
		if err != nil {
			return nil, refuse(RefuseNotANumber, "Number$")
		}
		return constV(int32(n)), nil
	case "SVar":
		name := strings.TrimSpace(expr)
		if !isName(name) {
			return nil, refuse(RefuseNotANumber, "SVar$")
		}
		return rd.name(name)
	}
	return nil, refuse(RefuseNeedsReference, head+"$"+leadWord(expr))
}

// name follows an SVar name to its body. X with no SVar named X is VX.
func (rd *valueReader) name(n string) (*ir.Value, *Refusal) {
	body, ok := rd.svars(n)
	if !ok {
		if n == "X" {
			return &ir.Value{Kind: ir.VX}, nil
		}
		return nil, refuse(RefuseSVarMissing, n)
	}
	if slices.Contains(rd.path, n) || len(rd.path) >= maxSVarDepth {
		return nil, refuse(RefuseSVarCycle, n)
	}
	rd.path = append(rd.path, n)
	defer func() { rd.path = rd.path[:len(rd.path)-1] }()
	return rd.text(body)
}

// op applies one /<op>[.<n>] suffix to v; n is any value text.
func (rd *valueReader) op(v *ir.Value, op string) (*ir.Value, *Refusal) {
	name, arg, hasArg := strings.Cut(strings.TrimSpace(op), ".")
	switch name {
	case "Twice", "Thrice", "HalfUp", "HalfDown":
		if hasArg {
			return nil, refuse(RefuseUnknownOperator, name+".")
		}
	}
	switch name {
	case "Twice":
		return binary(ir.VMul, constV(2), v), nil
	case "Thrice":
		return binary(ir.VMul, constV(3), v), nil
	case "HalfUp":
		return &ir.Value{Kind: ir.VHalfUp, Args: []*ir.Value{v}}, nil
	case "HalfDown":
		return &ir.Value{Kind: ir.VHalfDown, Args: []*ir.Value{v}}, nil
	}
	kind, ok := argOps[name]
	if !ok || !hasArg {
		return nil, refuse(RefuseUnknownOperator, name)
	}
	n, r := rd.text(arg)
	if r != nil {
		return nil, r
	}
	return binary(kind, v, n), nil
}

// argOps are the operators taking an operand. LimitMax caps the value (the
// lesser of the two), LimitMin floors it.
var argOps = map[string]ir.ValueKind{
	"Plus": ir.VAdd, "Minus": ir.VSub, "Times": ir.VMul, "LimitMax": ir.VMin, "LimitMin": ir.VMax,
}

func constV(n int32) *ir.Value { return &ir.Value{Kind: ir.VConst, N: n} }

func binary(k ir.ValueKind, a, b *ir.Value) *ir.Value {
	return &ir.Value{Kind: k, Args: []*ir.Value{a, b}}
}

// stampSrc records text on every value and test node of v that has none.
func stampSrc(v *ir.Value, text string) {
	if v == nil || v.Src != "" {
		return
	}
	v.Src = text
	for _, a := range v.Args {
		stampSrc(a, text)
	}
	if v.Test != nil {
		if v.Test.Src == "" {
			v.Test.Src = text
		}
		stampSrc(v.Test.L, text)
		stampSrc(v.Test.R, text)
	}
}

// isName reports whether s is an SVar name: a letter or '_', then letters,
// digits and '_'.
func isName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		letter := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// leadWord is s's leading run of letters and digits, for a refusal atom.
func leadWord(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return s[:i]
		}
	}
	return s
}
