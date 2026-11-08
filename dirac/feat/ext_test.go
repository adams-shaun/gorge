package feat

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

func extVanillaGame(seed uint64) (*dirac.State, dirac.RandomPolicy) {
	a, b := vanilla.Decks[seed%4], vanilla.Decks[(seed/4)%4]
	return dirac.NewConfig(vanilla.Table, a, b, seed, dirac.Config{}), dirac.NewRandomPolicy(seed * 7919)
}

func extVanillaEncoder() *Encoder {
	e := NewEncoder(NewPool(vanilla.Table, make([]CardSpec, len(vanilla.Table.Cards))))
	e.Reserve()
	return e
}

func extHas(x []Feature, slot int) (float32, bool) {
	for _, f := range x {
		if int(f.Row) == slot {
			return f.Value, true
		}
	}
	return 0, false
}

// TestExtTurnRows walks one random game: the mains-left slots follow the
// step rule at the active seat's decisions, predraw is set exactly in an
// untap/upkeep decision from turn 2 on, and never on turn 1.
func TestExtTurnRows(t *testing.T) {
	e := extVanillaEncoder()
	e.SetExt(ExtTurn)
	s, pol := extVanillaGame(3)
	var d dirac.Decision
	var predraws, turn1Upkeep int
	for n := 0; n < 20000; n++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		var p dirac.Projection
		s.Project(&p)
		st := e.EncodeState(s, &d)
		seat := d.Player
		mine := p.Active == seat
		step := stepIndex[p.Step]
		k := 0
		switch {
		case step <= 3:
			k = 2
		case step <= 9:
			k = 1
		}
		for j := 0; j < 6; j++ {
			_, got := extHas(st.Ext, 96+j)
			want := j == k && mine || j == 3+k && !mine
			if got != want {
				t.Fatalf("decision %d turn %d step %d: slot %d set=%v want %v", n, p.Turn, p.Step, 96+j, got, want)
			}
		}
		_, got := extHas(st.Ext, 102)
		_, gotOpp := extHas(st.Ext, 103)
		early := p.Step == kop.StepUntap || p.Step == kop.StepUpkeep
		if p.Turn <= 1 && early {
			turn1Upkeep++
		}
		if p.Turn <= 1 && (got || gotOpp) {
			t.Fatalf("predraw set on turn %d", p.Turn)
		}
		if want := mine && early && p.Turn >= 2; got != want {
			t.Fatalf("decision %d turn %d step %d: predraw me=%v want %v", n, p.Turn, p.Step, got, want)
		}
		if want := !mine && early && p.Turn >= 2; gotOpp != want {
			t.Fatalf("decision %d turn %d step %d: predraw opp=%v want %v", n, p.Turn, p.Step, gotOpp, want)
		}
		if got {
			predraws++
		}
		s.Apply(pol.Pick(&d))
	}
	t.Logf("predraw decisions %d, turn-1 early-step decisions %d", predraws, turn1Upkeep)
	if predraws == 0 {
		t.Fatal("no decision at an untap/upkeep step: the predraw rule was never exercised")
	}
}

const extLayoutFile = "testdata/extlayout.txt"

func TestExtLayoutFrozen(t *testing.T) {
	var b strings.Builder
	for i := 0; i < ExtWidth; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i, ExtSlotName(uint16(i)))
	}
	if os.Getenv("EXT_LAYOUT_REGEN") != "" {
		if err := os.WriteFile(extLayoutFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(extLayoutFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != b.String() {
		wl, gl := strings.Split(string(want), "\n"), strings.Split(b.String(), "\n")
		for i := range gl {
			if i >= len(wl) || wl[i] != gl[i] {
				t.Fatalf("slot line %d changed (bump ExtLayoutVersion, EXT_LAYOUT_REGEN=1): %q -> %q", i, wl[min(i, len(wl)-1)], gl[i])
			}
		}
		t.Fatal("layout file longer than the layout")
	}
	for i := 0; i < ExtWidth; i++ {
		n := ExtSlotName(uint16(i))
		switch {
		case i < 96 && !strings.HasPrefix(n, "E|") && n != "reserved":
			t.Errorf("slot %d (%s) in E's range", i, n)
		case i >= 96 && i < 112 && !strings.HasPrefix(n, "T|") && n != "reserved":
			t.Errorf("slot %d (%s) in T's range", i, n)
		case i >= 112 && strings.HasPrefix(n, "E|") || i >= 112 && strings.HasPrefix(n, "T|"):
			t.Errorf("slot %d (%s) outside its group", i, n)
		}
	}
	if ExtHash(ExtEconomy) == ExtHash(ExtEconomy|ExtTurn) || ExtLayoutVersion != 1 {
		t.Error("ExtHash does not name the groups")
	}
}

func TestExtEncodeAllocsNothing(t *testing.T) {
	e := extVanillaEncoder()
	e.SetExt(ExtAll)
	s, pol := extVanillaGame(5)
	var d dirac.Decision
	for n := 0; n < 60; n++ {
		s.StepInto(&d)
		s.Apply(pol.Pick(&d))
	}
	s.StepInto(&d)
	e.EncodeState(s, &d)
	if got := testing.AllocsPerRun(200, func() { e.EncodeState(s, &d) }); got != 0 {
		t.Fatalf("EncodeState with ExtAll allocates %v", got)
	}
	if len(e.EncodeState(s, &d).Ext) == 0 {
		t.Fatal("empty Ext: the allocation test measured nothing")
	}
}

// BenchmarkEncodeExt reports EncodeState on a vanilla position with the block off and on.
func BenchmarkEncodeExt(b *testing.B) {
	s, pol := extVanillaGame(5)
	var d dirac.Decision
	for n := 0; n < 60; n++ {
		s.StepInto(&d)
		s.Apply(pol.Pick(&d))
	}
	s.StepInto(&d)
	for _, c := range []struct {
		name string
		g    ExtGroups
	}{{"off", 0}, {"on", ExtEconomy | ExtTurn}} {
		b.Run(c.name, func(b *testing.B) {
			e := extVanillaEncoder()
			e.SetExt(c.g)
			e.EncodeState(s, &d)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.EncodeState(s, &d)
			}
		})
	}
}
