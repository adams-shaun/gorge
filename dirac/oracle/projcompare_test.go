package oracle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// projMutant is a kernel side whose projection is edited after it is taken: a
// field the lockstep compares then diverges.
type projMutant struct {
	*realSide
	edit func(*Projection)
}

func (m projMutant) Project(p *Projection) {
	m.realSide.Project(p)
	m.edit(p)
}

func mutateProjection(edit func(*Projection)) func(Side) Side {
	return func(s Side) Side { return projMutant{s.(*realSide), edit} }
}

// The kernel's stack entries carry the same modes and X as gorge's, and a
// kernel that recorded either wrongly would be caught.
func TestStackProjComparesModesAndX(t *testing.T) {
	if isolate(t) {
		return
	}
	const seeds = 64
	for _, c := range []struct {
		name, set string
		decks     [2][]cardset.Entry
		edit      func(*StackProj)
	}{
		{"X", "cmpheads", kcmpheadsDecks, func(st *StackProj) {
			if st.X != 0 {
				st.X++
			}
		}},
		{"Modes", "charm", chDecks, func(st *StackProj) {
			if st.Modes != 0 {
				st.Modes ^= 0xFF
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			lockstepRun(t, c.set, c.decks, seeds, true, nil)
			edit := mutateProjection(func(p *Projection) {
				for i := 0; i < int(p.NStack); i++ {
					c.edit(&p.Stack[i])
				}
			})
			if lockstepRunSide(t, c.set, c.decks, seeds, false, nil, edit) == 0 {
				t.Errorf("no seed in %d diverged with the kernel's %s wrong", seeds, c.name)
			}
		})
	}
}

// Both sides agree on each owner's exile order, and a kernel with two exiled
// cards in the wrong order would be caught.
func TestProjectionComparesExileOrder(t *testing.T) {
	if isolate(t) {
		return
	}
	const seeds = 64
	lockstepRun(t, "exilefd", kefdDecks, seeds, true, nil)
	edit := mutateProjection(func(p *Projection) {
		var first [2]*CardProj
		for i := range p.Cards {
			c := &p.Cards[i]
			if c.Zone != ZoneExile || c.Owner > 1 {
				continue
			}
			if f := first[c.Owner]; f != nil {
				f.Pos, c.Pos = c.Pos, f.Pos
				return
			}
			first[c.Owner] = c
		}
	})
	if lockstepRunSide(t, "exilefd", kefdDecks, seeds, false, nil, edit) == 0 {
		t.Errorf("no seed in %d diverged with two exiled cards swapped", seeds)
	}
}

// Every choose kind either side poses is in chooseKinds: a kind outside it
// shares the one choose:other code, which kfuzz -absolute fails as unknown.
// The translator lives in dirac/canon, so the census covers it too.
func TestChoosesHaveNoOther(t *testing.T) {
	if isolate(t) {
		return
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	canonFiles, err := filepath.Glob("../canon/*.go")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, canonFiles...)
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			lit, isLit := call.Args[0].(*ast.BasicLit)
			if !ok || !isLit || lit.Kind != token.STRING || fn.Name != "kind" && fn.Name != "ch" && fn.Name != "chooseCode" {
				return true
			}
			kind, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			if _, known := chooseCode(kind); !known {
				t.Errorf("%s: choose kind %q is not in chooseKinds", fset.Position(lit.Pos()), kind)
			}
			return true
		})
	}

	corpusDirOrSkip(t)
	for _, c := range []struct {
		set      string
		matchups [][2][]cardset.Entry
	}{
		{"manifest", kmanifestDecks},
		{"behold", kbeholdDecks},
		{"clonecopy", kclonecopyDecks},
		{"dskrooms", [][2][]cardset.Entry{kdrDecks}},
	} {
		set, err := cardset.RawByName(c.set)
		if err != nil {
			t.Fatal(err)
		}
		reg := scenRegistry(t)
		for _, md := range c.matchups {
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= 16; seed++ {
				m, err := NewMatchup(reg, [2]string{c.set + "-a", c.set + "-b"}, kcastDeck(t, reg, md), 1)
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40})
				CloseSide(k)
				if len(res.Stats.Unknown) != 0 {
					t.Errorf("%s seed %d: unknown option kinds %v", c.set, seed, res.Stats.Unknown)
				}
			}
		}
	}
}
