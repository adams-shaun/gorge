package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAnyPlayerActivationShapes: Activator$ Player lowers to
// kop.Ability.AnyActivator on any battlefield non-mana ability with a plain
// mana cost and no target or activation condition -- a Pump (Flailing Ogre),
// a Debuff (Ribbon Snake), a sorcery-speed Manifest (Sash and Waistcoat,
// Unmen's A[0], which keeps CostSorcery) -- and every shape the kernel's
// any-player offer does not model stays refused with an Activator$ reason.
func TestAnyPlayerActivationShapes(t *testing.T) {
	head := "Name:Any Probe\nManaCost:G\nTypes:Creature Probe\nPT:1/1\n"
	card := func(lines ...string) string { return head + strings.Join(lines, "\n") + "\nOracle:x\n" }
	for name, tc := range map[string]struct {
		src      string
		sorcery  bool
		wantAnyN int
	}{
		"Pump":     {src: card("A:AB$ Pump | Cost$ 1 | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | Activator$ Player | SpellDescription$ x.", "A:AB$ Pump | Cost$ 1 | Defined$ Self | NumAtt$ -1 | NumDef$ -1 | Activator$ Player | IsCurse$ True | SpellDescription$ x."), wantAnyN: 2},
		"Debuff":   {src: card("K:Flying", "A:AB$ Debuff | Cost$ 2 | Defined$ Self | Keywords$ Flying | Activator$ Player | SpellDescription$ x."), wantAnyN: 1},
		"Manifest": {src: card("A:AB$ Manifest | Cost$ 3 | Activator$ Player | SorcerySpeed$ True | SpellDescription$ x."), sorcery: true, wantAnyN: 1},
	} {
		d, rs := kwardpowBuild(t, "Any Probe", tc.src)
		if len(rs) != 0 {
			t.Errorf("%s: refused: %v", name, rs)
			continue
		}
		n := 0
		for _, a := range d.Abilities {
			if a.Kind != kop.AbActivated || !a.AnyActivator {
				continue
			}
			n++
			if got := a.Cost.Flags&kop.CostSorcery != 0; got != tc.sorcery {
				t.Errorf("%s: CostSorcery %v, want %v", name, got, tc.sorcery)
			}
		}
		if n != tc.wantAnyN {
			t.Errorf("%s: %d AnyActivator abilities, want %d (%+v)", name, n, tc.wantAnyN, d.Abilities)
		}
	}
	for name, src := range map[string]string{
		"Opponent":        card("A:AB$ Pump | Cost$ 1 | Defined$ Self | NumAtt$ +1 | Activator$ Opponent | SpellDescription$ x."),
		"Player.Opponent": card("A:AB$ Draw | Cost$ 1 | NumCards$ 1 | Activator$ Player.Opponent | SpellDescription$ x."),
		"You.NonActive":   card("A:AB$ Draw | Cost$ 1 | NumCards$ 1 | Activator$ You.NonActive | SpellDescription$ x."),
		"mana ability":    card("A:AB$ Mana | Cost$ SubCounter<1/CHARGE> | Produced$ C | Activator$ Player | SpellDescription$ x."),
		"pay-life cost":   card("A:AB$ Destroy | Cost$ PayLife<4> | Defined$ Self | Activator$ Player | SpellDescription$ x."),
		"sacrifice cost":  card("A:AB$ Draw | Cost$ 1 Sac<1/Land> | NumCards$ 1 | Activator$ Player | SpellDescription$ x."),
		"a target":        card("A:AB$ Pump | Cost$ 2 | ValidTgts$ Creature | KW$ Flying | Activator$ Player | SpellDescription$ x."),
		"PlayerTurn":      card("A:AB$ Destroy | Cost$ 2 | Defined$ Self | PlayerTurn$ True | Activator$ Player | SpellDescription$ x."),
		"IsPresent":       card("A:AB$ PutCounter | Cost$ 2 | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1 | IsPresent$ Card.Self+tapped | Activator$ Player | SpellDescription$ x."),
		"a once-per-turn": card("A:AB$ Pump | Cost$ 1 | Defined$ Self | NumAtt$ +1 | ActivationLimit$ 1 | Activator$ Player | SpellDescription$ x."),
		"sub-ability target": card("A:AB$ Pump | Cost$ 3 | Defined$ Self | Activator$ Player | SubAbility$ DBD | SpellDescription$ x.",
			"SVar:DBD:DB$ Destroy | ValidTgts$ Creature"),
	} {
		_, rs := kwardpowBuild(t, "Any Probe", src)
		if len(rs) == 0 {
			t.Errorf("%s: lowered, want a refusal", name)
			continue
		}
		hit := false
		for _, r := range rs {
			hit = hit || strings.Contains(r.Reason, "Activator")
		}
		if !hit {
			t.Errorf("%s: refused without an Activator$ reason: %v", name, rs)
		}
	}
}
