package kgen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExwOpTableCoversEveryOp: exwOwnerMover is default-refuse over
// exwOpTable, so an op the lowering emits that the table has no row for is
// silently over-refused (never silently allowed). This walks every `Op: "X"`
// literal and `.Op = "X"` assignment in kgen's non-test sources and fails
// until each op has a row, so the next added op is classified on purpose.
func TestExwOpTableCoversEveryOp(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		lit := func(e ast.Expr) {
			if b, ok := e.(*ast.BasicLit); ok && b.Kind == token.STRING {
				if s, err := strconv.Unquote(b.Value); err == nil {
					seen[s] = true
				}
			}
		}
		ast.Inspect(af, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.KeyValueExpr:
				if k, ok := n.Key.(*ast.Ident); ok && k.Name == "Op" {
					lit(n.Value)
				}
			case *ast.AssignStmt:
				if len(n.Lhs) == 1 && len(n.Rhs) == 1 {
					if s, ok := n.Lhs[0].(*ast.SelectorExpr); ok && s.Sel.Name == "Op" {
						lit(n.Rhs[0])
					}
				}
			}
			return true
		})
	}
	if len(seen) < 200 {
		t.Fatalf("walked only %d op literals: the census is not reading the lowering", len(seen))
	}
	var missing []string
	for op := range seen {
		if _, ok := exwOpTable[op]; !ok {
			missing = append(missing, op)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("ops with no exwOpTable row (lower_exwops.go): %v", missing)
	}
	// the ops this gate was found wrong on must stay refused
	for _, op := range []string{"Vote", "Discover", "DigImprint", "DelayedTrigger", "Return", "CastFree"} {
		if exwOpTable[op] != exwUnvetted {
			t.Errorf("%s is not exwUnvetted", op)
		}
	}
}

// TestExiledWithOwnerVoteDiscoverRefuse: Vote (the Council's-judgment shape
// exiles every most-voted permanent, so a tie exiles several owned by
// different players, each stamped ExiledWith by the source) and Discover
// (exiles the revealed cards) can link more than one card; the ref's scalar
// seat cannot represent that. Both refuse by name, and Hideaway's Play of
// the exiled card (C 1) is not an exile mover.
func TestExiledWithOwnerVoteDiscoverRefuse(t *testing.T) {
	vote := exiledWithOwnerDropETB(t, `T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigVote | TriggerDescription$ enters: will of the council, exile the most-voted permanent.
SVar:TrigVote:DB$ Vote | Defined$ Player | VoteCard$ Permanent.YouDontCtrl | VoteSubAbility$ DBExile
SVar:DBExile:DB$ ChangeZone | Defined$ Remembered | Origin$ Battlefield | Destination$ Exile`)
	discover := exiledWithOwnerDropETB(t, `T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDiscover | TriggerDescription$ enters: discover 4.
SVar:TrigDiscover:DB$ Discover | Num$ 4`)
	for _, tc := range []struct{ name, src, want string }{
		{"a vote exiling the most-voted", vote, "Vote"},
		{"a discover", discover, "Discover"},
	} {
		reg := testutil.CorpusRegistry(t)
		card, diags := cards.ParseBytes("exiled_with_owner_vote.txt", []byte(tc.src))
		if card == nil {
			t.Fatalf("%s: script did not parse: %v", tc.name, diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("%s: link: %v", tc.name, d)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		refused := false
		for _, rf := range lc.Refusals {
			if strings.Contains(rf.Reason, "ExiledWithSource player ref") && strings.Contains(rf.Reason, tc.want) {
				refused = true
			}
		}
		if !refused {
			t.Errorf("%s: lowered with no ExiledWithSource refusal naming %s: %v", tc.name, tc.want, lc.Refusals)
		}
	}
	if ok, err := exwOwnerMover(&lInstr{Op: "Hideaway", C: 1}, &lAbility{}); ok || err != nil {
		t.Errorf("hideaway Play: ok=%v err=%v, want neither", ok, err)
	}
	if ok, err := exwOwnerMover(&lInstr{Op: "Hideaway", A: cst(4)}, &lAbility{}); !ok || err != nil {
		t.Errorf("hideaway exile: ok=%v err=%v, want a mover", ok, err)
	}
}
