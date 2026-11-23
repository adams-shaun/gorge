package oracle

import (
	"testing"

	dK "github.com/adams-shaun/gorge/engine/decision"

	"github.com/adams-shaun/gorge/engine/state"
)

// Scratch: seed 1 plain, no Check hook.
func TestZZChooseTypeSeed1Plain(t *testing.T) {
	reg, set, gd, tbl := chooseTypeFixture(t)
	seed := uint64(1)
	m, err := NewMatchup(reg, [2]string{"choosetype-a", "choosetype-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := castFirstPolicy(seed)
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
	CloseSide(k)
	if res.Div != nil {
		t.Errorf("plain: diverged at %d: %s", res.Div.At, res.Div.Field)
	} else {
		t.Log("plain: clean")
	}
}

// Scratch: seed 1 with the trace Check hook.
func TestZZChooseTypeSeed1Trace(t *testing.T) {
	reg, set, gd, tbl := chooseTypeFixture(t)
	seed := uint64(1)
	m, err := NewMatchup(reg, [2]string{"choosetype-a", "choosetype-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := castFirstPolicy(seed)
	prev := ""
	n := 0
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
		Check: func(g *GorgeSide, ker Side) *Divergence {
			n++
			ct := ""
			for i := range g.E.G.Objs {
				o := &g.E.G.Objs[i]
				if o.ChosenType != "" {
					ct += " obj" + itoa2(int(o.ID)) + "=" + o.ChosenType
				}
			}
			if ct != prev {
				t.Logf("decision %d: chosen:%s", n, ct)
				prev = ct
			}
			if n == 1 {
				row := ""
				for i := 0; i < 13 && i < len(g.E.G.Objs); i++ {
					o := &g.E.G.Objs[i]
					nm := "?"
					if f := o.Face(); f != nil {
						nm = f.Name
					}
					row += " obj" + itoa2(i) + "=" + nm + "z" + itoa2(int(o.Zone))
				}
				t.Logf("gorge setup:%s", row)
			}
			if n >= 378 && n <= 381 {
				var gp, kp2 Projection
				project(g.E, &g.id, g.M, &gp)
				ker.Project(&kp2)
				row := ""
				for i := 0; i < 42; i++ {
					a, b := gp.Cards[i], kp2.Cards[i]
					row += " i" + itoa2(i) + ":r" + itoa2(int(a.Zone)) + "p" + itoa2(int(a.Pos)) + "/k" + itoa2(int(b.Zone)) + "p" + itoa2(int(b.Pos)) + "c" + itoa2(int(a.Card))
				}
				t.Logf("decision %d proj:%s", n, row)
			}
			if n >= 280 {
				o1 := g.E.G.Obj(1)
				if o1 != nil {
					t.Logf("decision %d: obj1 zone=%v chosen=%q", n, o1.Zone, o1.ChosenType)
				}
			}
			if n >= 360 {
				if d := g.Pending(); d != nil && d.Kind == dK.KTarget {
					src := g.E.G.Obj(d.Source)
					nm, ct := "?", ""
					if src != nil {
						nm, ct = src.Face().Name, src.ChosenType
					}
					t.Logf("decision %d: target ask source=%d %s chosen=%q opts=%v", n, d.Source, nm, ct, d.Options)
				}
			}
			if n >= 368 {
				row := ""
				for z, zn := range map[int]string{2: "bf", 3: "gy"} {
					for p := 0; p < 2; p++ {
						row += " " + zn + itoa2(p) + ":"
						for _, id := range g.E.G.Zone(state.Zone(z), state.PlayerID(p)) {
							o := g.E.G.Obj(id)
							nm := ""
							if f := o.Face(); f != nil {
								nm = f.Name
							}
							row += "#" + itoa2(int(id)) + nm + " "
						}
					}
				}
				t.Logf("decision %d state:%s", n, row)
			}
			if n >= 368 {
				if d := g.Pending(); d != nil {
					opts := ""
					for _, o := range d.Options {
						opts += " [" + itoa2(o.Index) + " " + o.Kind + " " + o.Label + "]"
					}
					t.Logf("decision %d: kind=%v resume=%s player=%d opts=%s", n, d.Kind, d.ResumeKind, d.Player, opts)
				} else {
					t.Logf("decision %d: no pending", n)
				}
			}
			return nil
		}})
	CloseSide(k)
	if res.Div != nil {
		t.Errorf("trace: diverged at %d: %s", res.Div.At, res.Div.Field)
	} else {
		t.Log("trace: clean")
	}
}

func itoa2(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
