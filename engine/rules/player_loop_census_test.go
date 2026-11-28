package rules

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The player-loop census (CR 101.4, CR 800.4a-800.4m): every place the rules
// engine walks the players is listed in testdata/player-loops.tsv with its
// order class and how it treats a player who left the game. The detector is
// syntactic on purpose: no type checking, no build tags, sorted file order.

// playerLoopHelperName matches helpers whose result is a player walk.
var playerLoopHelperName = regexp.MustCompile(`(?i)opponents?|eachplayer|apnap|turnorder|alivefrom`)

var (
	playerLoopOrders = map[string]bool{"apnap": true, "seat": true, "seat-visible": true, "n/a": true}
	playerLoopLefts  = map[string]bool{"skips": true, "includes-ok": true, "includes": true, "n/a": true}
)

// detectPlayerLoops returns the census keys of the player walks in one file,
// in source order. rel is the file's key prefix (its path under engine/).
func detectPlayerLoops(rel string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := playerLoopFuncName(fd)
		for n := range playerLoopSites(fd.Body) {
			keys = append(keys, fmt.Sprintf("%s:%s#%d", rel, name, n))
		}
	}
	return keys, nil
}

// playerLoopFuncName is "Recv.name" for a method, "name" for a function.
func playerLoopFuncName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		}
		break
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// playerLoopCallName is the called function's name, or "".
func playerLoopCallName(e ast.Expr) string {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return ""
	}
	switch fn := c.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.IndexExpr: // generic instantiation
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func isHelperWalkCall(e ast.Expr) bool {
	n := playerLoopCallName(e)
	return n != "" && playerLoopHelperName.MatchString(n)
}

// playerLoopSites returns the positions of the walks in one function body,
// in source order: a range over a .Players selector, a range over a helper
// result (the call, or a local holding it), a loop that indexes one, and a
// loop bounded by, or ranging over, the player count.
func playerLoopSites(body *ast.BlockStmt) []token.Pos {
	helperVars := playerHelperVars(body)
	isWalk := func(e ast.Expr) bool {
		e = unsliced(e)
		return isHelperWalkCall(e) || isNamedLocal(helperVars, e)
	}
	countVars := playerCountVars(body)
	var sites []token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.RangeStmt:
			sel, ok := ast.Unparen(s.X).(*ast.SelectorExpr)
			n := unconverted(s.X)
			counted := isPlayersLen(n) || isNamedLocal(countVars, n)
			if (ok && sel.Sel.Name == "Players") || counted || isWalk(s.X) || loopIndexesWalk(s, isWalk, s.Body) {
				sites = append(sites, s.Pos())
			}
		case *ast.ForStmt:
			if forBoundedByPlayerCount(s, countVars) || forUsesWalk(s, isWalk) {
				sites = append(sites, s.Pos())
			}
		}
		return true
	})
	return sites
}

func isNamedLocal(names map[string]bool, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && names[id.Name]
}

// unsliced strips slice expressions: opps[1:] walks what opps walks.
func unsliced(e ast.Expr) ast.Expr {
	for {
		s, ok := ast.Unparen(e).(*ast.SliceExpr)
		if !ok {
			return ast.Unparen(e)
		}
		e = s.X
	}
}

// localsMatching are the locals assigned, anywhere in the function, from an
// expression seed accepts or from another such local (to a fixpoint), each
// right-hand side first reduced by strip. Of a multi-value assignment
// (ps, ok := f()) the first local takes the match.
func localsMatching(body *ast.BlockStmt, strip func(ast.Expr) ast.Expr, seed func(ast.Expr) bool) map[string]bool {
	vars := map[string]bool{}
	for changed := true; changed; {
		changed = false
		mark := func(lhs ast.Expr, rhs ast.Expr) {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name == "_" || vars[id.Name] {
				return
			}
			if rhs = strip(rhs); seed(rhs) || isNamedLocal(vars, rhs) {
				vars[id.Name] = true
				changed = true
			}
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if len(s.Lhs) == len(s.Rhs) {
					for i := range s.Rhs {
						mark(s.Lhs[i], s.Rhs[i])
					}
				} else if len(s.Rhs) == 1 {
					mark(s.Lhs[0], s.Rhs[0])
				}
			case *ast.ValueSpec:
				if len(s.Names) == len(s.Values) {
					for i := range s.Values {
						mark(s.Names[i], s.Values[i])
					}
				} else if len(s.Values) == 1 {
					mark(s.Names[0], s.Values[0])
				}
			}
			return true
		})
	}
	return vars
}

// playerHelperVars are the locals holding a player-walk helper's result.
func playerHelperVars(body *ast.BlockStmt) map[string]bool {
	return localsMatching(body, unsliced, isHelperWalkCall)
}

// isPlayersLen reports whether e is len(<x>.Players).
func isPlayersLen(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return false
	}
	if id, ok := c.Fun.(*ast.Ident); !ok || id.Name != "len" {
		return false
	}
	sel, ok := ast.Unparen(c.Args[0]).(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Players"
}

// playerCountConversions are the type names a player count is converted to.
var playerCountConversions = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"PlayerID": true,
}

// unconverted strips one-argument integer conversions: state.PlayerID(x) is x.
func unconverted(e ast.Expr) ast.Expr {
	for {
		c, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok || len(c.Args) != 1 || !playerCountConversions[playerLoopCallName(c)] {
			return ast.Unparen(e)
		}
		e = c.Args[0]
	}
}

// playerCountVars are the locals holding the player count: assigned from
// len(<x>.Players), converted or not, or from another such local
// (pn := len(g.Players); zonePlayers := pn).
func playerCountVars(body *ast.BlockStmt) map[string]bool {
	return localsMatching(body, unconverted, isPlayersLen)
}

// forBoundedByPlayerCount reports whether a three-clause loop's condition
// compares against the player count (for p := 0; p < len(g.Players); p++).
func forBoundedByPlayerCount(s *ast.ForStmt, countVars map[string]bool) bool {
	if s.Cond == nil {
		return false
	}
	found := false
	ast.Inspect(s.Cond, func(n ast.Node) bool {
		if e, ok := n.(ast.Expr); ok && isPlayersLen(e) {
			found = true
		}
		if id, ok := n.(*ast.Ident); ok && countVars[id.Name] {
			found = true
		}
		return !found
	})
	return found
}

// forUsesWalk reports whether a three-clause loop's condition or body
// indexes a helper result or takes its length (i < len(opps)).
func forUsesWalk(s *ast.ForStmt, isWalk func(ast.Expr) bool) bool {
	nodes := []ast.Node{s.Body}
	if s.Cond != nil {
		nodes = append(nodes, s.Cond)
	}
	if loopIndexesWalk(s, isWalk, nodes...) {
		return true
	}
	return loopHas(s, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return false
		}
		id, ok := c.Fun.(*ast.Ident)
		return ok && id.Name == "len" && isWalk(c.Args[0])
	}, nodes...)
}

// loopIndexesWalk reports whether loop itself indexes a helper result.
func loopIndexesWalk(loop ast.Stmt, isWalk func(ast.Expr) bool, nodes ...ast.Node) bool {
	return loopHas(loop, func(n ast.Node) bool {
		ix, ok := n.(*ast.IndexExpr)
		return ok && isWalk(ix.X)
	}, nodes...)
}

// loopHas reports whether match holds somewhere in nodes, outside any loop
// nested in loop: the innermost loop is the walk's site.
func loopHas(loop ast.Stmt, match func(ast.Node) bool, nodes ...ast.Node) bool {
	found := false
	for _, root := range nodes {
		ast.Inspect(root, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.ForStmt, *ast.RangeStmt:
				if n != ast.Node(loop) {
					return false
				}
			}
			if n != nil && match(n) {
				found = true
			}
			return !found
		})
	}
	return found
}

// playerLoopCensusDirs are the trees the census covers, relative to this
// package's directory, with the key prefix (the tree's path under engine/).
var playerLoopCensusDirs = []struct{ dir, prefix string }{{".", "rules"}, {"../effects", "effects"}}

func scanPlayerLoops(t *testing.T) []string {
	t.Helper()
	type censusFile struct{ path, key string }
	var files []censusFile
	for _, d := range playerLoopCensusDirs {
		err := filepath.WalkDir(d.dir, func(p string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				if de.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				rel, err := filepath.Rel(d.dir, p)
				if err != nil {
					return err
				}
				files = append(files, censusFile{p, d.prefix + "/" + filepath.ToSlash(rel)})
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Slice(files, func(a, b int) bool { return files[a].key < files[b].key })
	var keys []string
	for _, f := range files {
		src, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		k, err := detectPlayerLoops(f.key, src)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k...)
	}
	return keys
}

type playerLoopRow struct{ order, left, note string }

func readPlayerLoopRows(t *testing.T, path string) map[string]playerLoopRow {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows := map[string]playerLoopRow{}
	sc := bufio.NewScanner(f)
	ln := 0
	for sc.Scan() {
		ln++
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			t.Errorf("%s:%d: want 4 tab-separated columns, got %d", path, ln, len(cols))
			continue
		}
		if _, dup := rows[cols[0]]; dup {
			t.Errorf("%s:%d: duplicate key %s", path, ln, cols[0])
		}
		rows[cols[0]] = playerLoopRow{cols[1], cols[2], cols[3]}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func readPlayerLoopKnown(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			keys = append(keys, l)
		}
	}
	return keys
}

func TestPlayerLoopsAreClassed(t *testing.T) {
	sites := scanPlayerLoops(t)
	rows := readPlayerLoopRows(t, "testdata/player-loops.tsv")
	known := readPlayerLoopKnown(t, "testdata/player-loops-known.txt")
	for i := 1; i < len(known); i++ {
		if known[i-1] >= known[i] {
			t.Errorf("testdata/player-loops-known.txt is not sorted without duplicates: %s follows %s", known[i], known[i-1])
		}
	}
	knownSet := map[string]bool{}
	for _, k := range known {
		knownSet[k] = true
	}
	seen := map[string]bool{}
	var visible, includes int
	for _, k := range sites {
		seen[k] = true
		r, ok := rows[k]
		if !ok {
			t.Errorf("player walk %s has no row in testdata/player-loops.tsv (class its order and left-the-game handling)", k)
			continue
		}
		if !playerLoopOrders[r.order] {
			t.Errorf("%s: order %q is not apnap|seat|seat-visible|n/a", k, r.order)
		}
		if !playerLoopLefts[r.left] {
			t.Errorf("%s: left %q is not skips|includes-ok|includes|n/a", k, r.left)
		}
		if r.left == "includes-ok" && strings.TrimSpace(r.note) == "" {
			t.Errorf("%s: includes-ok needs the reason in note", k)
		}
		bad := r.order == "seat-visible" || r.left == "includes"
		if r.order == "seat-visible" {
			visible++
		}
		if r.left == "includes" {
			includes++
		}
		if bad && !knownSet[k] {
			t.Errorf("%s is %s/%s and not in testdata/player-loops-known.txt: walk the players in turn order and skip players who left", k, r.order, r.left)
		}
	}
	for k := range rows {
		if !seen[k] {
			t.Errorf("row %s in testdata/player-loops.tsv names no player walk (remove it)", k)
		}
	}
	for _, k := range known {
		r, ok := rows[k]
		if !ok || !seen[k] || (r.order != "seat-visible" && r.left != "includes") {
			t.Errorf("%s is listed in testdata/player-loops-known.txt but is no longer seat-visible or includes: remove it (the list only shrinks)", k)
		}
	}
	t.Logf("player loops=%d seat-visible=%d includes=%d", len(sites), visible, includes)
	if os.Getenv("PLAYER_LOOPS_DUMP") != "" {
		for _, k := range sites {
			t.Log(k)
		}
	}
}

func TestPlayerLoopDetector(t *testing.T) {
	cases := []struct {
		file, src string
		want      []string
	}{
		{"rules/a.go", `package rules
func (e *Engine) untap() {
	for p := range e.G.Players {
		_ = p
	}
}`, []string{"rules/a.go:Engine.untap#0"}},
		{"rules/b.go", `package rules
func drainOpps(e *Engine, p int) {
	for _, q := range e.livingOpponents(p) {
		_ = q
	}
}`, []string{"rules/b.go:drainOpps#0"}},
		{"rules/c.go", `package rules
func cards(g *Game) {
	for _, id := range g.Players[0].Commanders {
		_ = id
	}
	for _, c := range g.Cards {
		_ = c
	}
}`, nil},
		// A multi-value helper result, aliased twice, then ranged.
		{"effects/d.go", `package effects
func flip(h Host, c *Ctx) {
	var players, again []int
	ps, ok := forEachPlayerFlippers(h, c)
	if ok {
		players = ps
		again = players
	}
	for _, p := range again {
		_ = p
	}
}`, []string{"effects/d.go:flip#0"}},
		// The player count held through a type conversion.
		{"rules/e.go", `package rules
func seats(e *Engine) {
	n := state.PlayerID(len(e.G.Players))
	for i := state.PlayerID(0); i < n; i++ {
		_ = i
	}
}`, []string{"rules/e.go:seats#0"}},
		// Indexing: in a range loop, in a range nested in a three-clause
		// loop (the innermost loop is the site), and of the call itself.
		// An index outside any loop is no walk.
		{"rules/f.go", `package rules
func idx(e *Engine, other []int) {
	opps := e.livingOpponents(0)
	_ = opps[0]
	for i := range other {
		_ = opps[i]
	}
	for j := 0; j < 3; j++ {
		for range other {
			_ = opps[j]
		}
	}
	for k := range other {
		_ = e.livingOpponents(0)[k]
	}
}`, []string{"rules/f.go:idx#0", "rules/f.go:idx#1", "rules/f.go:idx#2"}},
		// A range over the player count, direct or held in a local.
		{"rules/g.go", `package rules
func counted(g *Game) {
	for p := range len(g.Players) {
		_ = p
	}
	n := int(len(g.Players))
	for p := range state.PlayerID(n) {
		_ = p
	}
	m := uint8(n)
	for p := range m {
		_ = p
	}
}`, []string{"rules/g.go:counted#0", "rules/g.go:counted#1", "rules/g.go:counted#2"}},
		// A slice of a helper result, ranged directly or through an alias.
		{"rules/h.go", `package rules
func tail(g *Game) {
	alive := g.AliveFrom(0)
	for _, p := range alive[1:] {
		_ = p
	}
	rest := alive[1:]
	for _, p := range rest {
		_ = p
	}
}`, []string{"rules/h.go:tail#0", "rules/h.go:tail#1"}},
	}
	var got, want []string
	for _, c := range cases {
		k, err := detectPlayerLoops(c.file, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, k...)
		want = append(want, c.want...)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detector keys = %q, want %q", got, want)
	}
}
