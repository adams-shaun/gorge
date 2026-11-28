package custom

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// customRegisterSize is the number of rows in REGISTER.tsv. Raising it admits a
// card-specific node and needs the operator's sign-off; lower it when a row goes.
const customRegisterSize = 0

const registerHeader = "card\tscope\tclass\twhy"

// maxWhy bounds a row's reason: one sentence, in our own words.
const maxWhy = 160

// registerClasses is the closed list of what a custom node stands in for.
var registerClasses = []string{"effect", "condition", "cost", "static", "replacement", "trigger"}

// scopeRe is an ability's position on its face: kind letter and index.
var scopeRe = regexp.MustCompile(`^[atsr][0-9]+$`)

type row struct{ card, scope, class, why string }

// parseRegister parses and checks the register text: the header, four fields a
// row, a closed class, a well-formed scope, a short non-empty reason, rows
// sorted by (card, scope) with no pair repeated.
func parseRegister(text string) ([]row, error) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if lines[0] != registerHeader {
		return nil, fmt.Errorf("header is %q, want %q", lines[0], registerHeader)
	}
	var rows []row
	var errs []error
	for i, ln := range lines[1:] {
		n := i + 2
		f := strings.Split(ln, "\t")
		if len(f) != 4 {
			errs = append(errs, fmt.Errorf("line %d: %d fields, want 4", n, len(f)))
			continue
		}
		r := row{f[0], f[1], f[2], f[3]}
		if !contains(registerClasses, r.class) {
			errs = append(errs, fmt.Errorf("line %d: class %q is not one of %v", n, r.class, registerClasses))
		}
		if !scopeRe.MatchString(r.scope) {
			errs = append(errs, fmt.Errorf("line %d: scope %q does not match %s", n, r.scope, scopeRe))
		}
		if r.why == "" || len(r.why) > maxWhy {
			errs = append(errs, fmt.Errorf("line %d: why is %d bytes, want 1..%d", n, len(r.why), maxWhy))
		}
		if k := len(rows); k > 0 {
			p := rows[k-1]
			switch {
			case p.card == r.card && p.scope == r.scope:
				errs = append(errs, fmt.Errorf("line %d: (%q, %s) repeats", n, r.card, r.scope))
			case p.card > r.card || (p.card == r.card && p.scope > r.scope):
				errs = append(errs, fmt.Errorf("line %d: (%q, %s) is out of order: sort by card, then scope", n, r.card, r.scope))
			}
		}
		rows = append(rows, r)
	}
	return rows, errors.Join(errs...)
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// sizeViolation reports a row count that differs from the published size.
func sizeViolation(rows, size int) error {
	switch {
	case rows > size:
		return errors.New("a row was added: raising customRegisterSize needs the operator's sign-off")
	case rows < size:
		return fmt.Errorf("a row was deleted: lower customRegisterSize to %d", rows)
	}
	return nil
}

func TestCustomRegisterOnlyShrinks(t *testing.T) {
	b, err := os.ReadFile("REGISTER.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseRegister(string(b))
	if err != nil {
		t.Fatalf("REGISTER.tsv: %v", err)
	}
	if err := sizeViolation(len(rows), customRegisterSize); err != nil {
		t.Errorf("REGISTER.tsv has %d rows, customRegisterSize is %d: %v", len(rows), customRegisterSize, err)
	}

	// The checks, on in-memory registers.
	h := registerHeader + "\n"
	good := h + "Alpha\ta0\teffect\tA reason.\nAlpha\ta1\tcost\tA reason.\nBeta\tt0\ttrigger\tA reason.\n"
	if rows, err := parseRegister(good); err != nil || len(rows) != 3 {
		t.Fatalf("good register: %d rows, err %v", len(rows), err)
	}
	if rows, err := parseRegister(h); err != nil || len(rows) != 0 {
		t.Fatalf("header-only register: %d rows, err %v", len(rows), err)
	}
	for _, c := range []struct{ name, text, want string }{
		{"header", "card\tscope\tclass\n", "header"},
		{"three fields", h + "Alpha\ta0\teffect\n", "3 fields"},
		{"five fields", h + "Alpha\ta0\teffect\tA reason.\textra\n", "5 fields"},
		{"class", h + "Alpha\ta0\tmana\tA reason.\n", `class "mana"`},
		{"scope letter", h + "Alpha\tk0\teffect\tA reason.\n", `scope "k0"`},
		{"scope no index", h + "Alpha\ta\teffect\tA reason.\n", `scope "a"`},
		{"unsorted card", h + "Beta\ta0\teffect\tA reason.\nAlpha\ta0\teffect\tA reason.\n", "out of order"},
		{"unsorted scope", h + "Alpha\ta1\teffect\tA reason.\nAlpha\ta0\teffect\tA reason.\n", "out of order"},
		{"repeat", h + "Alpha\ta0\teffect\tA reason.\nAlpha\ta0\tcost\tA reason.\n", "repeats"},
		{"empty why", h + "Alpha\ta0\teffect\t\n", "why is 0 bytes"},
		{"long why", h + "Alpha\ta0\teffect\t" + strings.Repeat("x", maxWhy+1) + "\n", "why is 161 bytes"},
	} {
		if _, err := parseRegister(c.text); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one containing %q", c.name, err, c.want)
		}
	}
	if _, err := parseRegister(h + "Alpha\ta0\teffect\t" + strings.Repeat("x", maxWhy) + "\n"); err != nil {
		t.Errorf("a %d-byte why must pass: %v", maxWhy, err)
	}
	for _, c := range []struct {
		rows, size int
		want       string
	}{
		{1, 0, "a row was added: raising customRegisterSize needs the operator's sign-off"},
		{2, 3, "a row was deleted: lower customRegisterSize to 2"},
	} {
		if err := sizeViolation(c.rows, c.size); err == nil || err.Error() != c.want {
			t.Errorf("rows %d size %d: err = %v, want %q", c.rows, c.size, err, c.want)
		}
	}
	if err := sizeViolation(4, 4); err != nil {
		t.Errorf("equal count: %v", err)
	}
}
