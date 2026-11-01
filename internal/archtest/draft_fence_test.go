package archtest

import (
	"sort"
	"strings"
	"testing"
)

const draftTree = module + "/draft"

// draftFenceMinPackages is TestDraftTreeStaysAbove's anti-vacuous floor.
const draftFenceMinPackages = 50

// draftImportersAllowed are the only packages outside draft/ that may depend on
// it: the commands that drive it. cmd/gorged links the plugin; server/gorged
// itself reaches draft only through Extensions{Events}.
var draftImportersAllowed = map[string]bool{
	module + "/cmd/gorged":   true,
	module + "/cmd/draftsim": true,
	module + "/cmd/labdemo":  true,
	module + "/cmd/extagent": true,
}

// draftFenceViolations names every transitive dependency on draft/ from a
// package outside it that draftImportersAllowed does not list. Test imports are
// out of scope, as they are for packages.
func draftFenceViolations(pkgs map[string]pkg) []string {
	var out []string
	for path, p := range pkgs {
		if path == draftTree || strings.HasPrefix(path, draftTree+"/") || draftImportersAllowed[path] {
			continue
		}
		for dep := range p.deps {
			if dep == draftTree || strings.HasPrefix(dep, draftTree+"/") {
				out = append(out, path+" depends on "+dep)
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestDraftTreeStaysAbove: draft/ sits above the engine, the server library, the
// bots and the test kit, and nothing below it may reach up. It requires the draft
// packages to exist, so a rename cannot leave it passing over nothing.
func TestDraftTreeStaysAbove(t *testing.T) {
	pkgs := packages(t)
	for _, must := range []string{draftTree + "/gorgedevents", draftTree + "/eventhost", draftTree + "/limited/draft"} {
		if _, ok := pkgs[must]; !ok {
			t.Fatalf("%s is not in go list's output: the fence would run vacuously", must)
		}
	}
	if len(pkgs) < draftFenceMinPackages {
		t.Fatalf("only %d packages to check, fewer than %d: the fence would run vacuously", len(pkgs), draftFenceMinPackages)
	}
	if bad := draftFenceViolations(pkgs); len(bad) > 0 {
		t.Errorf("packages reach up into draft/:\n\t%s", strings.Join(bad, "\n\t"))
	}
}

// TestDraftFenceViolationsSynthetic pins the helper on a package map small enough
// to read: the library server may not depend on draft/, its command may.
func TestDraftFenceViolationsSynthetic(t *testing.T) {
	event := draftTree + "/event"
	got := draftFenceViolations(map[string]pkg{
		module + "/server/gorged": {deps: set(event)},
		module + "/cmd/gorged":    {deps: set(event)},
		draftTree + "/eventhost":  {deps: set(event)},
		module + "/engine/rules":  {deps: set(module + "/draftish")},
	})
	want := module + "/server/gorged depends on " + event
	if len(got) != 1 || got[0] != want {
		t.Errorf("draftFenceViolations = %q, want only %q", got, want)
	}
}
