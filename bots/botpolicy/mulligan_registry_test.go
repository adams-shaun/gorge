package botpolicy

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestMulliganBuiltinsHaveStableIDs pins the registry's built-in ids: the
// existing constants keep their meaning, so every stored/zero-value
// MulliganRule is unchanged.
func TestMulliganBuiltinsHaveStableIDs(t *testing.T) {
	builtins := []struct {
		id   MulliganRule
		name string
	}{
		{MulliganCoin, "coin"},
		{MulliganLands, "lands"},
		{MulliganNever, "never"},
		{MulliganConstructed, "constructed"},
		{MulliganForge, "forge"},
	}
	for i, b := range builtins {
		if int(b.id) != i {
			t.Fatalf("builtin %s: id %d, want %d", b.name, b.id, i)
		}
		if got := b.id.String(); got != b.name {
			t.Fatalf("MulliganRule(%d).String() = %q, want %q", i, got, b.name)
		}
		if _, err := MulliganByName(b.name); err != nil {
			t.Fatalf("MulliganByName(%q): %v", b.name, err)
		} else if rule, _ := MulliganByName(b.name); rule != b.id {
			t.Fatalf("MulliganByName(%q) = %d, want %d", b.name, rule, b.id)
		}
	}
}

// TestMulliganNamesSorted asserts the listing is sorted and carries the
// built-ins.
func TestMulliganNamesSorted(t *testing.T) {
	names := MulliganNames()
	if !slices.IsSorted(names) {
		t.Fatalf("MulliganNames not sorted: %v", names)
	}
	for _, want := range []string{"coin", "lands", "never", "constructed", "forge"} {
		if !slices.Contains(names, want) {
			t.Fatalf("MulliganNames missing %q: %v", want, names)
		}
	}
}

// TestMulliganByNameUnknownListsValid asserts the error names every valid
// policy.
func TestMulliganByNameUnknownListsValid(t *testing.T) {
	_, err := MulliganByName("no-such-policy")
	if err == nil {
		t.Fatal("MulliganByName(unknown) returned no error")
	}
	for _, name := range MulliganNames() {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not list registered policy %q", err.Error(), name)
		}
	}
}

// mullPolicy is the test plugin: mulligan whenever the ask offers a
// mulligan, keep otherwise (and bottom the default way when the seat has
// kept).
type mullPolicy struct{ name string }

func (p mullPolicy) Name() string { return p.name }
func (p mullPolicy) Answer(b Board, d *decision.Decision) ([]int, bool) {
	for _, o := range d.Options {
		if o.Kind == "mulligan" {
			return []int{o.Index}, true
		}
	}
	if d.Options[0].Kind == "bottom" {
		return b.chooseDiscard(d), true
	}
	return []int{d.Options[0].Index}, true
}

// TestMulliganPluginProof registers another policy, selects it BY NAME onto
// a Board, and shows a KMulligan decision is answered by it while the
// lands rule on the same hand keeps. No non-test file knows the plugin.
func TestMulliganPluginProof(t *testing.T) {
	id := RegisterMulligan(mullPolicy{name: "always-mull"})
	if got := MulliganNames(); !slices.Contains(got, "always-mull") {
		t.Fatalf("registry after plugin registration: %v", got)
	}
	rule, err := MulliganByName("always-mull")
	if err != nil {
		t.Fatalf("MulliganByName(always-mull): %v", err)
	}
	if rule != id {
		t.Fatalf("MulliganByName(always-mull) = %d, want id %d", rule, id)
	}

	// A hand the lands rule keeps (3 lands): precondition for "the two
	// answers differ, so the plugin's answer can only come from the plugin".
	lands := handBoard(MulliganLands, sevenWith(3)...)
	d := keepMullDecision(0, true)
	var mullOpt *decision.Option
	for i := range d.Options {
		if d.Options[i].Kind == "mulligan" {
			mullOpt = &d.Options[i]
		}
	}
	if mullOpt == nil {
		t.Fatal("precondition: decision offers no mulligan option")
	}
	lc, lok := lands.mulliganByRule(&d)
	if !lok || !slices.Equal(lc, []int{0}) {
		t.Fatalf("precondition: lands rule answered %+v (ok %v), want keep [0]", lc, lok)
	}

	plugin := handBoard(rule, sevenWith(3)...)
	pc, pok := plugin.mulliganByRule(&d)
	if !pok {
		t.Fatal("plugin policy did not answer the keep/mulligan ask")
	}
	if !slices.Equal(pc, []int{mullOpt.Index}) {
		t.Fatalf("plugin answered %v, want mulligan %d", pc, mullOpt.Index)
	}

	// The plugin's id is usable on a fresh Board: string round-trips.
	if plugin.Mulligan.String() != "always-mull" {
		t.Fatalf("plugin id String = %q, want always-mull", plugin.Mulligan.String())
	}
}

// TestMulliganDuplicateRegistrationPanics asserts the registry rejects a
// duplicate name.
func TestMulliganDuplicateRegistrationPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration did not panic")
		}
	}()
	RegisterMulligan(mullPolicy{name: "coin"})
}

// TestMulliganEmptyNameRegistrationPanics asserts the registry rejects an
// empty name.
func TestMulliganEmptyNameRegistrationPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("empty-name registration did not panic")
		}
	}()
	RegisterMulligan(mullPolicy{name: ""})
}
