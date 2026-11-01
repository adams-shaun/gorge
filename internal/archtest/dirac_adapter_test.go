package archtest

import "testing"

// TestAdapterLinksNoRulesEngine holds a served dirac game solo: nothing the adapter
// links can reach gorge's rules engine.
func TestAdapterLinksNoRulesEngine(t *testing.T) {
	requireDirac(t)
	pkgs := packages(t)
	seen := map[string]bool{}
	var walk func(string)
	walk = func(p string) {
		if seen[p] {
			return
		}
		seen[p] = true
		if q, ok := pkgs[p]; ok {
			for imp := range q.imports {
				walk(imp)
			}
		}
	}
	root := module + "/dirac/adapter"
	if _, ok := pkgs[root]; !ok {
		t.Fatal("no dirac/adapter package")
	}
	walk(root)
	for _, banned := range []string{"/engine/rules", "/engine/replay", "/dirac/oracle"} {
		if seen[module+banned] {
			t.Errorf("dirac/adapter links %s", module+banned)
		}
	}
}
