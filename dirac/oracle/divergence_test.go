package oracle

import (
	"path/filepath"
	"testing"
)

// attackBlind is a faulty kernel: a gorge side whose projection never shows
// a creature attacking, so the first committed attack diverges.
type attackBlind struct{ *GorgeSide }

func (k attackBlind) Project(p *Projection) {
	k.GorgeSide.Project(p)
	for i := range p.Cards {
		p.Cards[i].Flags &^= FAttacking
	}
}

// noBlocks is a faulty kernel that never offers a block.
type noBlocks struct{ *GorgeSide }

func (k noBlocks) Step() CDecision {
	d := k.GorgeSide.Step()
	var keep []Canon
	for _, a := range d.Options {
		if a.Kind() != ActBlock {
			keep = append(keep, a)
		}
	}
	d.Options = keep
	return d
}

// TestDivergenceReproShrink injects two kernel faults and checks that
// lockstep finds each, that the repro round-trips through JSON and re-runs
// to the same divergence, and that the shrink pass keeps the divergence
// class while dropping prefix actions.
func TestDivergenceReproShrink(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	cfg := RunConfig{Config: Config{PassElision: true}}
	for _, tc := range []struct {
		name  string
		wrap  func(*GorgeSide) Side
		class string
	}{
		{"attack-blind", func(g *GorgeSide) Side { return attackBlind{g} }, "projection Cards[].Flags"},
		{"no-blocks", func(g *GorgeSide) Side { return noBlocks{g} }, "options options"},
	} {
		mk := func(m *Matchup, seed uint64) (Side, error) {
			g, err := NewGorgeSide(m, seed, cfg.Config, nil)
			if err != nil {
				return nil, err
			}
			return tc.wrap(g), nil
		}
		var rep *Repro
		for _, p := range Pairings(pool, 8, 99) {
			m, err := c.Matchup(p.A, p.B, 0)
			if err != nil {
				t.Fatal(err)
			}
			k, _ := mk(m, p.Seed)
			res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), cfg)
			if res.Div == nil {
				continue
			}
			if got := res.Div.fieldClass(); got != tc.class {
				t.Fatalf("%s: divergence class %q, want %q (%v)", tc.name, got, tc.class, res.Div)
			}
			rep = NewRepro(m, p.Seed, cfg, &res)
			break
		}
		if rep == nil {
			t.Fatalf("%s: no divergence in 8 games", tc.name)
		}
		path := filepath.Join(t.TempDir(), "repro.json")
		if err := rep.Write(path); err != nil {
			t.Fatal(err)
		}
		back, err := ReadRepro(path)
		if err != nil {
			t.Fatal(err)
		}
		m, err := back.Matchup(c.Reg)
		if err != nil {
			t.Fatal(err)
		}
		re := back.Rerun(m, mk, 0)
		if re.Div == nil || re.Div.fieldClass() != tc.class || re.Div.At != rep.Divergence.At {
			t.Fatalf("%s: rerun %v, recorded %v", tc.name, re.Div, rep.Divergence)
		}
		sh := Shrink(c, back, mk, 60)
		if sh.Divergence == nil || sh.Divergence.fieldClass() != tc.class || len(sh.Actions) > len(rep.Actions) {
			t.Fatalf("%s: shrink lost the divergence or grew: %v, %d -> %d", tc.name, sh.Divergence, len(rep.Actions), len(sh.Actions))
		}
		t.Logf("%s: diverged at %d (%s), shrunk %d -> %d actions in %d attempts", tc.name, rep.Divergence.At, rep.Divergence.Field,
			len(rep.Actions), len(sh.Actions), sh.Shrunk.Attempts)
	}
}
