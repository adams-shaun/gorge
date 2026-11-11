package kgen

import (
	"strings"
	"testing"
)

// TestXColorActivatedCost: an activated ability's XColor$ (a Crypt
// Rats-shaped "Spend only black mana on X") lowers to Cost.XColor, 1 + the
// colour; a letter spelling reads the same; a multi-colour value leaves the
// X generic; a spell's XColor$ stays unread (gorge reads it on no spell), and
// a part beside the {X} other than mana and {T} refuses by name.
func TestXColorActivatedCost(t *testing.T) {
	const head = "Name:X Probe\nManaCost:2 B\nTypes:Creature Rat\nPT:1/1\n"
	ab := func(cost, col string) string {
		return head + "A:AB$ DamageAll | Cost$ " + cost + " | XColor$ " + col +
			" | NumDmg$ X | ValidCards$ Creature | ValidPlayers$ Player\nSVar:X:Count$xPaid\n"
	}
	for _, p := range []struct {
		name, src string
		want      uint8
	}{
		{"black word", ab("X", "Black"), 3},
		{"red letter beside T", ab("T X", "R"), 4},
		{"two colours", ab("X", "Black,Red"), 0},
	} {
		d, rs := kwardpowBuild(t, "X Probe", p.src)
		if len(rs) != 0 {
			t.Fatalf("%s: refused: %v", p.name, rs)
		}
		if len(d.Abilities) == 0 || d.Abilities[0].Cost.Mana.X != 1 {
			t.Fatalf("%s: no {X} activation lowered", p.name)
		}
		if got := d.Abilities[0].Cost.XColor; got != p.want {
			t.Errorf("%s: Cost.XColor %d, want %d", p.name, got, p.want)
		}
	}
	for _, p := range []struct{ name, src, want string }{
		{"spell", "Name:X Probe\nManaCost:X B\nTypes:Sorcery\nA:SP$ DealDamage | XColor$ Black | ValidTgts$ Any | NumDmg$ X\nSVar:X:Count$xPaid\n", "unread params [XColor]"},
		{"beside a sacrifice", ab("X Sac<1/Creature>", "Black"), "XColor$ Black beside a cost part"},
	} {
		_, rs := kwardpowBuild(t, "X Probe", p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}
}
