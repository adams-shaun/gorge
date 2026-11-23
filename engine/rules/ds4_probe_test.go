package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// ds4 scratch probe (never commit): does the priority walk offer a plain
// Flash cast of Dust Elemental under the repro's board (Manabarbs live,
// payer at 1 life, the pool the two engines disagree about)?
func ds4DustBoard(t *testing.T, life int32, pool state.Mana, barbs bool) (*Engine, state.ObjID, string) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, 4242, "Name:Dust Elemental\nManaCost:2 W W\nTypes:Creature Elemental\nPT:6/6\nK:Flash\nK:Flying\nK:Fear\nOracle:x\n")
	for i := 0; i < 2; i++ {
		id := onBoardCard(t, e, 0, corpusCard(t, "Plains"))
		e.G.Obj(id).SummonSick = false
	}
	for i := 0; i < 3; i++ {
		id := onBoardCard(t, e, 0, corpusCard(t, "Island"))
		e.G.Obj(id).SummonSick = false
	}
	if barbs {
		bid := onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
		if o := e.G.Obj(bid); o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Manabarbs zone %v", o.Zone)
		}
	}
	e.G.Players[0].Life = life
	e.G.Players[0].Pool = pool
	if o := e.G.Obj(spell); o.Zone != state.ZHand {
		t.Fatalf("precondition: spell zone %v", o.Zone)
	}
	d := ppPriority(t, e)
	var casts []string
	for _, o := range d.Options {
		if o.Kind == "cast" {
			casts = append(casts, o.Label)
		}
	}
	joined := "none"
	if len(casts) > 0 {
		joined = casts[0]
		for _, c := range casts[1:] {
			joined += "|" + c
		}
	}
	return e, spell, joined
}

func TestDS4ProbeDustOffer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		life  int32
		pool  state.Mana
		barbs bool
	}{
		{"plain life20 nopool", 20, state.Mana{}, false},
		{"plain life1 nopool", 1, state.Mana{}, false},
		{"barbs life20 nopool", 20, state.Mana{}, true},
		{"barbs life1 nopool", 1, state.Mana{}, true},
		{"barbs life1 poolC", 1, state.Mana{state.MC: 1}, true},
		{"barbs life1 poolWUU", 1, func() state.Mana { m := state.Mana{}; m[state.MW] = 1; m[state.MU] = 2; return m }(), true},
		{"barbs life1 pool2W2U", 1, func() state.Mana { m := state.Mana{}; m[state.MW] = 2; m[state.MU] = 2; return m }(), true},
		{"plain life20 pool2W2U", 20, func() state.Mana { m := state.Mana{}; m[state.MW] = 2; m[state.MU] = 2; return m }(), false},
	}
	var lines []string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, spell, got := ds4DustBoard(t, tc.life, tc.pool, tc.barbs)
			_ = e
			want := false
			for _, lbl := range ds4Split(got) {
				if lbl == "Cast Dust Elemental" {
					want = true
				}
			}
			lines = append(lines, ds4Sprintf(tc.name, tc.life, tc.pool, tc.barbs, want, got, spell))
		})
	}
	t.Fatalf("probe results:\n%s", ds4Join(lines))
}

func ds4Sprintf(name string, life int32, pool state.Mana, barbs, want bool, got string, spell state.ObjID) string {
	return name + ": life=" + ds4Itoa(life) + " barbs=" + ds4Bs(barbs) + " offered=" + ds4Bs(want) + " casts=" + got + " spell=" + ds4Itoa(int32(spell)) + "\n"
}

func ds4Itoa(n int32) string { return fmt.Sprintf("%d", n) }

func ds4Bs(b bool) string {
	if b {
		return "T"
	}
	return "F"
}

func ds4Join(lines []string) string {
	s := ""
	for _, l := range lines {
		s += l
	}
	return s
}

func ds4Split(s string) []string {
	if s == "none" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
