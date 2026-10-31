package gorgegame

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func sampleConfig(t *testing.T) game.Config {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 2)
	return game.Config{Seed: 7, Names: names, PlayerNames: names, Decks: decks, Mulligans: 1}
}

func firstLegal(d *decision.Decision) decision.Intent {
	ch := make([]int, 0, d.Min)
	for i := 0; i < d.Min && i < len(d.Options); i++ {
		ch = append(ch, d.Options[i].Index)
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
}

// playOut answers every decision with its first legal choice until the game
// ends, calling each after every accepted answer.
func playOut(t *testing.T, g game.Game, each func()) {
	t.Helper()
	for d := g.Pending(); d != nil; d = g.Pending() {
		if err := g.Submit(firstLegal(d)); err != nil {
			t.Fatalf("submit %d: %v", d.Seq, err)
		}
		if each != nil {
			each()
		}
	}
	if !g.Board().Over {
		t.Fatal("no pending decision on an unfinished game")
	}
}

func TestGorgeGameMatchesEngine(t *testing.T) {
	g, err := Engine{}.New(sampleConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	e := Rules(g)
	if e == nil {
		t.Fatal("Rules is nil on a gorge game")
	}

	d := g.Pending()
	bad := firstLegal(d)
	bad.Seq++
	head := g.Log().Head()
	if err := g.Submit(bad); err == nil {
		t.Fatal("a stale intent was accepted")
	}
	if g.Pending() != d || g.Log().Head() != head {
		t.Fatal("a refused intent moved the game")
	}

	fork := g.Fork()
	if Rules(fork) == e || fork.Board() == g.Board() || fork.Log() == g.Log() {
		t.Fatal("Fork shares the engine")
	}

	playOut(t, g, func() {
		if g.Board() != e.G || g.Log() != e.L || g.Chars() != view.Chars(e) || g.Pending() != e.Pending() {
			t.Fatal("the game reads something other than its engine")
		}
	})
	if fork.Log().Head() != head {
		t.Fatal("playing the original moved its fork")
	}

	want := game.Caps{Engine: "gorge", SubBurstViews: true, ManualMana: true, SearchRoots: true, StartingAsk: true}
	if got := g.Caps(); got != want {
		t.Fatalf("Caps = %+v", got)
	}
	if got := (Engine{}).Name(); got != want.Engine {
		t.Fatalf("Name = %q", got)
	}
}

func TestRulesIsNilOnAnotherEngine(t *testing.T) {
	var other struct{ game.Game }
	if Rules(other) != nil {
		t.Fatal("Rules answered for a game it does not run")
	}
}

// TestGorgeEngineNewMatchesHostBuild holds Engine.New to the sequence a hosted
// match builds its engine with: the starting-player constructor, the object
// cap, a reserved log, the ask, then Advance.
func TestGorgeEngineNewMatchesHostBuild(t *testing.T) {
	c := sampleConfig(t)
	direct := rules.NewStartingPlayerChoice(rules.Config{Seed: c.Seed, Names: c.Names, PlayerNames: c.PlayerNames,
		Decks: c.Decks, Mulligans: c.Mulligans, LoopGuard: rules.WithDefaultObjectCap(nil)})
	direct.L.Reserve(80000)
	direct.AskStartingPlayer()
	direct.Advance()

	c.ExpectedEvents = 80000
	g, err := Engine{}.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if g.Log().Head() != direct.L.Head() {
		t.Fatalf("genesis head %s, host build %s", g.Log().Head(), direct.L.Head())
	}
	if !reflect.DeepEqual(g.Pending(), direct.Pending()) {
		t.Fatalf("first decision %+v, host build %+v", g.Pending(), direct.Pending())
	}
	playOut(t, g, func() {
		if err := direct.Submit(firstLegal(direct.Pending())); err != nil {
			t.Fatal(err)
		}
	})
	if g.Log().Head() != direct.L.Head() {
		t.Fatalf("final head %s, host build %s", g.Log().Head(), direct.L.Head())
	}
}

func TestGorgeEngineReplays(t *testing.T) {
	c := sampleConfig(t)
	var en Engine
	g, err := en.New(c)
	if err != nil {
		t.Fatal(err)
	}
	playOut(t, g, nil)
	l := g.Log()

	back, err := en.Replay(l, c)
	if err != nil {
		t.Fatal(err)
	}
	if back.Log().Head() != l.Head() {
		t.Fatalf("replayed head %s, played %s", back.Log().Head(), l.Head())
	}

	if _, err := en.ReplayTo(l, c, 1); err != nil {
		t.Fatal(err)
	}
	var visited []int
	if _, err := en.Walk(l, c, 2, func(g game.Game, i int) error {
		if Rules(g) == nil {
			t.Fatal("Walk visited a game with no engine")
		}
		visited = append(visited, i)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(visited, []int{0, 1, 2}) {
		t.Fatalf("visited %v", visited)
	}

	cut := l.Clone()
	cut.Events = append(cut.Events[:len(cut.Events):len(cut.Events)], events.Event{Seq: uint64(len(cut.Events)), Kind: events.PlayerLost})
	stopped, err := en.Replay(cut, c)
	var d *game.Divergence
	if !errors.As(err, &d) || !d.Short || stopped == nil {
		t.Fatalf("a longer recording: game %v, err %v", stopped, err)
	}

	if g, err := en.Replay(nil, c); err == nil || g != nil {
		t.Fatalf("nil log: game %v, err %v", g, err)
	}
}

// fill sets v to a value no zero Config holds.
func fill(t *testing.T, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.Uint64:
		v.SetUint(1)
	case reflect.Int, reflect.Int32:
		v.SetInt(1)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.String:
		v.SetString("x")
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
	default:
		t.Fatalf("fill: no case for %s", v.Kind())
	}
}

// rulesOnly names the rules.Config fields game.Config does not carry, and why
// each is safe to leave out.
var rulesOnly = map[string]string{
	"PlanarDecks": "no host table plays Planechase",
	"NamedCorpus": "the oracle harness's own corpus",
	"Spare":       "a batch runner's recycled storage",
	"LoopGuard":   "derived from GuardDisabled and NoObjectCap",
}

// gameOnly names the game.Config fields with no rules.Config counterpart.
var gameOnly = map[string]bool{"GuardDisabled": true, "NoObjectCap": true, "Key": true}

func TestRulesConfigCarriesEveryField(t *testing.T) {
	var c game.Config
	cv := reflect.ValueOf(&c).Elem()
	for i := range cv.NumField() {
		fill(t, cv.Field(i))
	}
	got := reflect.ValueOf(RulesConfig(c))

	shared := map[string]bool{}
	for i := range cv.NumField() {
		name := cv.Type().Field(i).Name
		rf := got.FieldByName(name)
		if !rf.IsValid() {
			if !gameOnly[name] {
				t.Errorf("game.Config.%s has no rules.Config field and is not listed in gameOnly", name)
			}
			continue
		}
		shared[name] = true
		if rf.IsZero() {
			t.Errorf("RulesConfig drops %s", name)
			continue
		}
		if name == "Format" {
			continue // a distinct type, checked below
		}
		if !reflect.DeepEqual(rf.Interface(), cv.Field(i).Interface()) {
			t.Errorf("RulesConfig changes %s", name)
		}
	}

	for i := range got.NumField() {
		name := got.Type().Field(i).Name
		_, listed := rulesOnly[name]
		if shared[name] == listed {
			t.Errorf("rules.Config.%s: carried %v, listed in rulesOnly %v; exactly one must hold", name, shared[name], listed)
		}
	}

	for f, want := range map[game.Format]rules.Format{game.FormatConstructed: rules.FormatConstructed, game.FormatCommander: rules.FormatCommander} {
		if got := RulesConfig(game.Config{Format: f}).Format; got != want {
			t.Errorf("format %d maps to %d, want %d", f, got, want)
		}
	}

	if g := RulesConfig(game.Config{GuardDisabled: true}).LoopGuard; g == nil || !g.Disabled {
		t.Errorf("GuardDisabled: LoopGuard %+v", g)
	}
	if g := RulesConfig(game.Config{}).LoopGuard; g == nil || g.Disabled || g.MaxObjs != rules.DefaultMaxObjs {
		t.Errorf("default: LoopGuard %+v", g)
	}
	if g := RulesConfig(game.Config{NoObjectCap: true}).LoopGuard; g != nil {
		t.Errorf("NoObjectCap: LoopGuard %+v", g)
	}

	// A field the host sets but game.Config cannot carry would be lost on the
	// way to the engine, so no host file may name one.
	files, err := filepath.Glob("../../../server/host/*.go")
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	note := func(e ast.Expr) {
		if id, ok := e.(*ast.Ident); ok {
			named[id.Name] = true
		}
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				note(n.Sel)
			case *ast.KeyValueExpr:
				note(n.Key)
			}
			return true
		})
	}
	if !named["NameUniverseNames"] {
		t.Fatal("the host scan saw no rules.Config field; it read the wrong files")
	}
	for name, why := range rulesOnly {
		if name != "LoopGuard" && named[name] {
			t.Errorf("server/host names %s, which game.Config does not carry (%s)", name, why)
		}
	}
}

func TestGameConfigInvertsRulesConfig(t *testing.T) {
	var c game.Config
	cv := reflect.ValueOf(&c).Elem()
	for i := range cv.NumField() {
		fill(t, cv.Field(i))
	}
	c.Key = ""
	for _, guard := range []struct{ disabled, noCap bool }{{false, false}, {true, false}, {false, true}} {
		c.GuardDisabled, c.NoObjectCap = guard.disabled, guard.noCap
		if got := GameConfig(RulesConfig(c)); !reflect.DeepEqual(got, c) {
			t.Errorf("guard %+v: %+v, want %+v", guard, got, c)
		}
	}
}
