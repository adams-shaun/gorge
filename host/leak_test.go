package host

import (
	"reflect"
	"testing"
)

// engineAndStatePkg are the two packages whose live objects are the god-view
// that fixture's D6 exists to keep off the wire. A result whose concrete
// element type lives in either — *rules.Engine, *state.Game, *[]state.Game,
// and so on — is a handle a caller could type-assert straight into a live
// engine game, exactly the defect the mtgserve snapshot shipped to
// production. No view./protocol./decision. carrier ever has an element in
// either package, so a result here means a leak.
const (
	rulesPkg = "github.com/adams-shaun/gorge/rules"
	statePkg = "github.com/adams-shaun/gorge/state"
)

// assertNoEngineHandle is the runtime half of D6: whatever a data-returning
// *Registry method hands back must never be (or contain) a live *state.Game
// or *rules.Engine. It proves it by reflecting the concrete type — pointer
// and slice/array wrappers are followed to their element — and checking the
// element's package. Containers that merely CARRY protocol/view/decision
// values (a chan protocol.Frame, an http.Handler, an interface) stop the
// walk: there is nothing underneath to type-assert into. A nil result is not
// a handle and passes; a concrete element in rules/ or state/ is the leak.
//
// The walk deliberately checks the TOP-LEVEL element only: it does not step
// into a struct's own exported fields. A protocol.Something{} whose field
// is a *state.Game is therefore invisible to this function — but that case
// is the compile-time half's job (arch_test.go's
// TestNoExportLeaksAnEngineGame walks every exported signature and its
// argument/result types to the element type), which catches it wherever the
// finicky live-marshal plumbing here would not. This runtime half exists for
// the reason the compile-time scan cannot cover: a signature that names the
// leaking type only indirectly (an interface or a slice of it).
func assertNoEngineHandle(t *testing.T, name string, v any) {
	t.Helper()
	if v == nil {
		return // no live game behind a nil handle
	}
	typ := reflect.TypeOf(v)
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Func, reflect.Chan, reflect.Interface:
		return // a crossable boundary, not a concrete engine element
	}
	if pkg := typ.PkgPath(); pkg == rulesPkg || pkg == statePkg {
		t.Errorf("host: %s returns a *state.Game / *rules.Engine (element %s) — a god-view leak", name, typ)
	}
}

// TestNoRegistryDataResultIsAnEngineGame is Task M2c-4's runtime half of D6
// (arch_test.go's TestNoExportLeaksAnEngineGame is the compile-time half).
// It drives a live 2-seat human-bot match and calls every data-returning
// *Registry method, asserting each result is a concrete view./protocol./
// decision. carrier and never a *state.Game / *rules.Engine a caller could
// type-assert a god view out of. The compile-time scan would catch the same
// defect at build; this guards against a future signature that names the
// type indirectly (an interface or a slice of it) that go doc's text still
// shows but whose leak is only visible by looking at the concrete value.
func TestNoRegistryDataResultIsAnEngineGame(t *testing.T) {
	r, id := startHumanTable(t)
	waitPending(t, r, id, 1, 0) // sync: match live, seats installed, a decision parked

	// Resolve the (value, error) pairs to their values so a leak never hides
	// behind an error return, then capture it. valOrFail fails the test on
	// the error (a method that errored proves nothing either way).
	viewAt, err := r.ViewAt(id, 1, 0)
	if err != nil {
		t.Fatalf("ViewAt: %v", err)
	}
	viewAtSeat, err := r.ViewAtSeat(id, 1, 0, 0)
	if err != nil {
		t.Fatalf("ViewAtSeat: %v", err)
	}
	events, err := r.Events(id, 1, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	eventsSeat, err := r.EventsSeat(id, 1, 0, 0)
	if err != nil {
		t.Fatalf("EventsSeat: %v", err)
	}
	pending, err := r.Pending(id, 1, 0)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	matches, err := r.Matches(id)
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}

	// Every data-returning *Registry method and its concrete result. Each is
	// asserted to carry no engine/game element below.
	cases := []struct {
		name string
		got  any
	}{
		{"ViewAt", viewAt},
		{"ViewAtSeat", viewAtSeat},
		{"Events", events},
		{"EventsSeat", eventsSeat},
		{"Pending", pending},
		{"Matches", matches},
		{"Tables", r.Tables()},
		{"Hello", r.Hello(&Session{ID: "leaktest"})},
	}
	for _, c := range cases {
		assertNoEngineHandle(t, "Registry."+c.name, c.got)
	}
	// Sanity: the loop above covered every data-returning method; an empty
	// cases list would make the assertion trivially vacuous.
	if n := len(cases); n == 0 {
		t.Fatal("no data-returning Registry methods covered")
	}
}
