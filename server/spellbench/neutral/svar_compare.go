package neutral

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// svarFromFace resolves a named SVar body through the count evaluator,
// including the Count$Compare head. The body's operand and branch names are
// looked up in the same printed face, so a value the native engine computes
// from the ask's controller (Frost Bite's "3 if you control three or more
// snow permanents, else 2") resolves the same way here instead of leaving the
// target effect's damage unknown.
func (s *Session) svarFromFace(o *protocol.Observation, src *protocol.ObjectRef, f *cards.Face, name string, depth int) (int32, bool) {
	if f == nil || depth > 8 {
		return 0, false
	}
	body := strings.TrimSpace(f.SVars[name])
	if body == "" {
		return 0, false
	}
	if rest, ok := strings.CutPrefix(body, "Count$Compare "); ok {
		return s.compareValue(o, src, f, rest, depth)
	}
	return s.svarValue(o, src, body)
}

// compareValue mirrors the engine's evalCompare grammar:
// `<name> <OP><threshold>.<ifTrue>.<ifFalse>`, where every operand is an
// integer literal, an SVar name on the face, or an empty branch. A token that
// resolves to neither is unknown, never a confident branch.
func (s *Session) compareValue(o *protocol.Observation, src *protocol.ObjectRef, f *cards.Face, arg string, depth int) (int32, bool) {
	name, rest, _ := strings.Cut(arg, " ")
	rest = strings.TrimSpace(rest)
	if name == "" || len(rest) < 2 {
		return 0, false
	}
	op, tail := rest[:2], rest[2:]
	thTok, branches, _ := strings.Cut(tail, ".")
	if thTok == "" {
		return 0, false
	}
	ifTok, elseTok, _ := strings.Cut(branches, ".")
	value, ok := s.compareOperand(o, src, f, name, depth+1)
	if !ok {
		return 0, false
	}
	th, ok := s.compareOperand(o, src, f, thTok, depth+1)
	if !ok {
		return 0, false
	}
	var hit bool
	switch op {
	case "GE":
		hit = value >= th
	case "GT":
		hit = value > th
	case "EQ":
		hit = value == th
	case "LE":
		hit = value <= th
	case "LT":
		hit = value < th
	default:
		return 0, false
	}
	if hit {
		return s.compareOperand(o, src, f, ifTok, depth+1)
	}
	return s.compareOperand(o, src, f, elseTok, depth+1)
}

// compareOperand resolves one Compare operand: an integer literal, an empty
// token (the engine's zero branch), or a named SVar on the face.
func (s *Session) compareOperand(o *protocol.Observation, src *protocol.ObjectRef, f *cards.Face, tok string, depth int) (int32, bool) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return 0, true
	}
	if n, err := strconv.Atoi(tok); err == nil {
		return int32(n), true
	}
	if _, ok := f.SVars[tok]; ok {
		return s.svarFromFace(o, src, f, tok, depth)
	}
	return 0, false
}
