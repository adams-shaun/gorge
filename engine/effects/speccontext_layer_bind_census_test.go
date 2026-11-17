package effects

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// specContextBindCensusFile is the allowlist TestSpecContextLayerBindCensus
// holds: one "<engine-relative file> <Func>" line per function that builds a
// SpecContext, reads it through MatchesObjectCtx/MatchesSpecCtx and does not
// bind the on-demand layer tables (specLayerWants / bindSpecLayerTables /
// layerTablesFor / a LayerTables host call) itself. Matching is any
// MatchesObjectCtx/MatchesSpecCtx entry, the unexported filter cores
// (matchesZoneSpecCtx, matchesObjectPtr, compiledMatchZone), or a
// zoneCountFold literal (the Count$Valid scan).
const specContextBindCensusFile = "testdata/speccontext_layer_bind.txt"

// bindCalls are the calls that count as binding a layer table: the one
// text-to-tables rule, its two callers, and the host read itself. The census
// is per function and does not track WHICH tables a LayerTables(want) call
// asks for: a site that binds one table but reads another is a review check
// (specLayerWants pins the spec-to-tables rule in TestSpecLayerWants).
var bindCalls = map[string]bool{
	"specLayerWants": true, "bindSpecLayerTables": true, "layerTablesFor": true,
	"LayerTables": true,
	// rules' own table builders, read directly where a rules site binds a table.
	"derivedColorTable": true, "EffectiveKeywords": true, "cdaPTTable": true,
	"battlefieldPTTable": true, "staticallyGoaded": true,
}

// matchCalls are the entry points a SpecContext is built for.
var matchCalls = map[string]bool{
	"MatchesObjectCtx": true, "MatchesSpecCtx": true, "MatchesSpecCtxPtr": true,
	// the unexported filter cores a fold or scan reaches a SpecContext through.
	"matchesZoneSpecCtx": true, "matchesObjectPtr": true, "compiledMatchZone": true,
}

// matchHolders are types whose literal carries a SpecContext's match into
// methods of another function: building one is a match site. zoneCountFold is
// the Count$Valid scan evalCountBodyZone hands its specCtx to (visit matches).
var matchHolders = map[string]bool{"zoneCountFold": true}

// specContextSites returns the census keys of every non-test function under
// engine/ that constructs a SpecContext (a SpecContext{...} literal or a
// call to a SpecContext/specCtx-named builder) and calls a match entry point, and whether each
// also binds a layer table. The scan is per function declaration (function
// literals count toward their enclosing declaration): a context handed to a
// helper in another function is that helper's site.
func specContextSites(t *testing.T) map[string]bool {
	t.Helper()
	sites := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, "../"))
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			var builds, matches, binds bool
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CompositeLit:
					builds = builds || typeName(x.Type) == "SpecContext"
					matches = matches || matchHolders[typeName(x.Type)]
				case *ast.CallExpr:
					switch name := typeName(x.Fun); {
					case matchCalls[name]:
						matches = true
					case bindCalls[name]:
						binds = true
					case buildsSpecContext(name):
						builds = true
					}
				}
				return true
			})
			if builds && matches {
				key := rel + " " + funcKey(fd)
				sites[key] = sites[key] || binds
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sites
}

// buildsSpecContext reports whether a called name constructs a SpecContext:
// Ctx.SpecContext, the rules engine's specCtx/specCtxSVars/rememberedSpecContext
// family and their kin all carry the substring in their name.
func buildsSpecContext(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "specctx") || strings.Contains(l, "speccontext")
}

// typeName is the last identifier of an expression (`effects.SpecContext`,
// `c.SpecContext`, `SpecContext` all read "SpecContext").
func typeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

func funcKey(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	rt := fd.Recv.List[0].Type
	if s, ok := rt.(*ast.StarExpr); ok {
		rt = s.X
	}
	return typeName(rt) + "." + fd.Name.Name
}

// TestSpecContextLayerBindCensus is the ratchet behind specLayerWants (CR
// 613.3): a SpecContext built outside Resolve binds only the always-published
// layer-3/4 tables, so a count or match over it reads the PRINTED colour,
// keyword or P/T unless its builder binds the layered one. Every function that
// builds a SpecContext and reads it through MatchesObjectCtx/MatchesSpecCtx
// either binds through specLayerWants (the tables its spec can read, in one
// host call) or is listed in testdata/speccontext_layer_bind.txt. A new
// unlisted site fails (bind it, or list it with the reason in the commit); a
// listed site that now binds, or no longer exists, fails as stale.
func TestSpecContextLayerBindCensus(t *testing.T) {
	sites := specContextSites(t)
	if len(sites) < 5 {
		t.Fatalf("census found %d sites; the scan is broken", len(sites))
	}
	if os.Getenv("PINS_RECORD") != "" {
		regenSpecContextBindCensus(t, sites)
	}
	listed := map[string]bool{}
	fh, err := os.Open(specContextBindCensusFile)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		listed[line] = true
	}
	var unlisted, stale []string
	for key, binds := range sites {
		if !binds && !listed[key] {
			unlisted = append(unlisted, key)
		}
	}
	for key := range listed {
		if binds, ok := sites[key]; !ok {
			stale = append(stale, key+" (no longer builds and matches a SpecContext)")
		} else if binds {
			stale = append(stale, key+" (now binds its layer tables)")
		}
	}
	sort.Strings(unlisted)
	sort.Strings(stale)
	for _, k := range unlisted {
		t.Errorf("unlisted SpecContext site %s: bind its tables through specLayerWants/bindSpecLayerTables, or add it to %s", k, specContextBindCensusFile)
	}
	for _, k := range stale {
		t.Errorf("stale %s entry: %s", specContextBindCensusFile, k)
	}
}

// TestSpecLayerWants pins the text-to-tables rule the bind census asks sites to
// share.
func TestSpecLayerWants(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want LayerTableSet
	}{
		{"Creature.Green", LayerColors},
		{"Creature.withFlying", LayerKeywords},
		{"Creature.powerGE3", LayerCDAPT | LayerBattlefieldPT},
		{"Creature.IsGoaded", LayerGoads},
		{"Creature.YouCtrl", 0},
		{"Creature.Green+withFlying", LayerColors | LayerKeywords},
	} {
		if got := specLayerWants(tc.spec); got != tc.want {
			t.Errorf("specLayerWants(%q) = %b, want %b", tc.spec, got, tc.want)
		}
	}
}

// regenSpecContextBindCensus rewrites the allowlist to the current unbound
// sites (PINS_RECORD=1): the list may only shrink in review, so
// a regeneration that grows it is the diff to question.
func regenSpecContextBindCensus(t *testing.T, sites map[string]bool) {
	t.Helper()
	var keys []string
	for key, binds := range sites {
		if !binds {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	const header = `# Functions that build a SpecContext, read it through MatchesObjectCtx /
# MatchesSpecCtx, and do not bind the on-demand layer tables themselves
# (specLayerWants / bindSpecLayerTables / layerTablesFor / a LayerTables host
# call, or a rules-side table read). TestSpecContextLayerBindCensus holds this
# list: a new site fails until it binds or is listed; a site that now binds, or
# is gone, fails as stale. The list only shrinks as sites move onto
# bindSpecLayerTables. Format: <engine-relative file> <Func>.
`
	if err := os.WriteFile(specContextBindCensusFile, []byte(header+strings.Join(keys, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
