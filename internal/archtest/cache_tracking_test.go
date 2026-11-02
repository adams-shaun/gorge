package archtest

import "testing"

// TestOpenPackageDirsCoversEveryPackage guards the cache-correctness wiring.
// The go-list-based checks read
// the module through a `go list` subprocess, whose reads go test's result
// cache cannot see; packages() opens every package directory from the test
// process (through openModulePackageDirs) so the cache tracks the source (see
// packages' doc comment).
//
// The live case asserts the wiring itself, not the helper in isolation:
// openModuleDirsN records how many directories openModulePackageDirs opened
// the one time packages() ran in this test binary, and the test holds that
// against the package count packages() returned. Removing the
// openModulePackageDirs call from packages() — the refactor that would
// silently reopen the stale-cache hole — leaves the count at zero and fails
// here, instead of leaving the hole open behind a green run. (The helper's
// own guard subtests below stay: they pin the conditions that would make the
// open vacuous or partial if the wiring is present.)
//
// The failure modes pinned are exactly the ones that would silently reopen
// the stale-cache hole:
//
//   - packages() did not open the package directories at all (wiring dropped);
//   - an empty package map (the open would run vacuously);
//   - a package with no directory (the open would be skipped for it).
//
// It must be able to fail: a regression in any of the above fails here rather
// than being served a stale cached pass.
func TestOpenPackageDirsCoversEveryPackage(t *testing.T) {
	pkgs := packages(t)
	if len(pkgs) < 10 {
		t.Fatalf("packages() returned only %d packages; the guard would run vacuously", len(pkgs))
	}
	if openModuleDirsN != len(pkgs) {
		t.Errorf("packages() opened %d directories for %d packages through openModulePackageDirs; a dropped or partial open leaves the stale-cache hole open", openModuleDirsN, len(pkgs))
	}
	if _, err := openPackageDirs(map[string]pkg{}); err == nil {
		t.Error("openPackageDirs accepted an empty package map; the cache-tracking open would run vacuously")
	}
	if _, err := openPackageDirs(map[string]pkg{"x": {path: "x"}}); err == nil {
		t.Error("openPackageDirs accepted a package with no directory; the cache-tracking open would be skipped for it")
	}
}
