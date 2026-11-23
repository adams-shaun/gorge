package manifest

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
)

// StatusKind is an entry's state of activation.
type StatusKind uint8

const (
	// Reserved: the code exists and nothing implements it.
	Reserved StatusKind = iota
	// Active: the kernel implements the code.
	Active
	// Blocked: the code waits on a named design (objspace, unbounded, ...).
	Blocked
	// Deferred: the code waits on a named decision.
	Deferred
)

var statusWords = [...]string{Reserved: "reserved", Active: "active", Blocked: "blocked", Deferred: "deferred"}

func (k StatusKind) String() string { return statusWords[k] }

// Status is an entry's status line: the kind and, for blocked and deferred,
// why.
type Status struct {
	Kind StatusKind
	Why  string
}

func (s Status) String() string {
	if s.Why == "" {
		return s.Kind.String()
	}
	return s.Kind.String() + " " + s.Why
}

// ParseStatus reads a status line's value.
func ParseStatus(v string) (Status, error) {
	word, why, _ := strings.Cut(v, " ")
	why = strings.TrimSpace(why)
	for k, w := range statusWords {
		if word != w {
			continue
		}
		s := Status{Kind: StatusKind(k), Why: why}
		switch s.Kind {
		case Reserved, Active:
			if why != "" {
				return Status{}, fmt.Errorf("status %s takes no reason, got %q", word, why)
			}
		default:
			if why == "" {
				return Status{}, fmt.Errorf("status %s needs a reason: %q", word, word+" <why>")
			}
		}
		return s, nil
	}
	return Status{}, fmt.Errorf("status %q is not reserved, active, blocked <why> or deferred <why>", v)
}

// Entry is one manifest file.
type Entry struct {
	// Path is the file's slash path from the module root,
	// dirac/manifest/<Family>/<Name>.
	Path   string
	Family string
	Name   string
	// Value is the code; for a Next entry it is provisional (Resolve) until
	// Assign writes it.
	Value int64
	// Next marks "value: next".
	Next     bool
	Approval string
	Forge    []string
	Census   string
	State    string
	// Contexts are the matcher kinds a FilterWord supports (a FilterWord
	// entry only): kgen refuses the word in a matcher not listed.
	Contexts []string
	Status   Status
	Ticket   string
	Doc      string

	raw []byte // the file as read
}

// Fam is e's family (nil only for an entry that failed to parse).
func (e *Entry) Fam() *Family { return FamilyNamed(e.Family) }

// RefusalClass is the kgen refusal class of a table code that names e,
// "manifest:<status>:<Family>/<Name>", or "" when e is active (invariant 3:
// no inactive code reaches a table).
func (e *Entry) RefusalClass() string {
	if e.Status.Kind == Active {
		return ""
	}
	return "manifest:" + e.Status.Kind.String() + ":" + e.Family + "/" + e.Name
}

// fieldKeys are the keys an entry may hold.
var fieldKeys = []string{"family", "name", "value", "approval", "forge", "census", "state", "contexts", "status", "ticket", "doc"}

// ParseEntry reads one entry file; path is its slash path from the module
// root. It checks the syntax only (Validate checks the rest).
func ParseEntry(path string, b []byte) (*Entry, error) {
	e := &Entry{Path: path, raw: b}
	seen := map[string]bool{}
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fail := func(format string, a ...any) error {
			return fmt.Errorf("%s:%d: %s", path, i+1, fmt.Sprintf(format, a...))
		}
		if strings.ContainsRune(line, '\r') {
			return nil, fail("carriage return")
		}
		key, val, ok := strings.Cut(line, ":")
		val = strings.TrimSpace(val)
		if !ok || key != strings.TrimSpace(key) || val == "" {
			return nil, fail("want \"key: value\", got %q", line)
		}
		known := false
		for _, k := range fieldKeys {
			known = known || k == key
		}
		if !known {
			return nil, fail("unknown key %q (keys: %s)", key, strings.Join(fieldKeys, ", "))
		}
		if seen[key] {
			return nil, fail("%s given twice", key)
		}
		seen[key] = true
		switch key {
		case "family":
			e.Family = val
		case "name":
			e.Name = val
		case "value":
			if val == "next" {
				e.Next = true
				break
			}
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil, fail("value %q is not a decimal integer or next", val)
			}
			e.Value = v
		case "approval":
			e.Approval = val
		case "forge":
			for _, tok := range strings.Split(val, "|") {
				tok = strings.TrimSpace(tok)
				if tok == "" {
					return nil, fail("empty forge token in %q", val)
				}
				e.Forge = append(e.Forge, tok)
			}
		case "census":
			e.Census = val
		case "state":
			e.State = val
		case "contexts":
			for _, c := range strings.Split(val, ",") {
				e.Contexts = append(e.Contexts, strings.TrimSpace(c))
			}
		case "status":
			s, err := ParseStatus(val)
			if err != nil {
				return nil, fail("%v", err)
			}
			e.Status = s
		case "ticket":
			e.Ticket = val
		case "doc":
			e.Doc = val
		}
	}
	for _, k := range []string{"family", "name", "value", "status"} {
		if !seen[k] {
			return nil, fmt.Errorf("%s: no %s line", path, k)
		}
	}
	return e, nil
}

// checkSelf holds e to the rules it can be checked on alone: its place in the
// tree, its family, its name and its value's range.
func (e *Entry) checkSelf() error {
	fail := func(format string, a ...any) error {
		return fmt.Errorf("%s: %s", e.Path, fmt.Sprintf(format, a...))
	}
	f := e.Fam()
	if f == nil {
		return fail("family %q is not a manifest family", e.Family)
	}
	if want := Dir + "/" + e.Family + "/" + e.Name; e.Path != want {
		return fail("family %s and name %s belong in %s", e.Family, e.Name, want)
	}
	if !token.IsIdentifier(e.Name) {
		return fail("name %q is not a Go identifier", e.Name)
	}
	if exported := token.IsExported(e.Name); exported != (f.Pkg == "kop") {
		return fail("name %s: a %s constant (package %s) must be %s", e.Name, f.Name, f.Pkg, map[bool]string{true: "exported", false: "unexported"}[f.Pkg == "kop"])
	}
	if !strings.HasPrefix(e.Name, f.Prefix) || len(e.Name) == len(f.Prefix) {
		return fail("name %s: a %s name starts with %q", e.Name, f.Name, f.Prefix)
	}
	if !e.Next {
		pos := e.Value
		if f.Bits {
			if pos = bitOf(e.Value); pos < 0 {
				return fail("value %d: a %s value is a single bit", e.Value, f.Name)
			}
		}
		if pos < f.Min || pos > f.Max {
			what := "value"
			if f.Bits {
				what = "bit"
			}
			return fail("%s %d is outside %s's range [%d,%d]", what, pos, f.Name, f.Min, f.Max)
		}
		if b := f.reserved(pos); b != nil && e.Approval == "" {
			return fail("value %d is in %s's reserved band [%d,%d] (%s): add an approval line", e.Value, f.Name, b.Lo, b.Hi, b.Why)
		}
	}
	if f.check != nil {
		if err := f.check(e); err != nil {
			return fail("%v", err)
		}
	}
	if len(e.Contexts) > 0 && f.Name != "FilterWord" {
		return fail("a contexts line is a FilterWord's: %s is a %s", e.Name, f.Name)
	}
	return nil
}

// FilterContexts are the matcher kinds a FilterWord's contexts line may name:
// the permanent matcher, the card matcher off the battlefield, the card
// matcher on the stack and the layer-walk matcher (dirac/kop/filterprog.go).
var FilterContexts = []string{"perm", "card", "spell", "cur"}

// checkFilterWord is the FilterWord family's own entry rule: a contexts line
// naming each matcher kind once, from FilterContexts.
func checkFilterWord(e *Entry) error {
	if len(e.Contexts) == 0 {
		return fmt.Errorf("a FilterWord needs a contexts line (%s)", strings.Join(FilterContexts, ","))
	}
	seen := map[string]bool{}
	for _, c := range e.Contexts {
		ok := false
		for _, w := range FilterContexts {
			ok = ok || c == w
		}
		if !ok {
			return fmt.Errorf("context %q is not one of %s", c, strings.Join(FilterContexts, ","))
		}
		if seen[c] {
			return fmt.Errorf("context %s given twice", c)
		}
		seen[c] = true
	}
	return nil
}

// Format is e as an entry file, its lines in the canonical order (family,
// name, value, approval, forge, census, state, contexts, status, ticket, doc), empty
// ones left out: what a tool writing a new entry (seed, adopt) writes.
func (e *Entry) Format() []byte {
	v := "next"
	if !e.Next {
		v = strconv.FormatInt(e.Value, 10)
	}
	var b strings.Builder
	for _, f := range []struct{ k, v string }{
		{"family", e.Family}, {"name", e.Name}, {"value", v}, {"approval", e.Approval},
		{"forge", strings.Join(e.Forge, " | ")}, {"census", e.Census}, {"state", e.State},
		{"contexts", strings.Join(e.Contexts, ",")},
		{"status", e.Status.String()}, {"ticket", e.Ticket}, {"doc", e.Doc},
	} {
		if f.v != "" {
			b.WriteString(f.k + ": " + f.v + "\n")
		}
	}
	return []byte(b.String())
}
