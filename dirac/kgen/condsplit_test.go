package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCondSplitKgenLowers: a Condition-split mana ability (River of Tears' land
// drop, Gemstone Caverns' luck counter; lower_condsplit.go) lowers as ONE mana
// ability carrying the root's mana and the arm's gate, colours and counter, and
// no stray program for the DB$ Mana sub. Every variant beyond the two shapes
// refuses with a reason naming the part.
func TestCondSplitKgenLowers(t *testing.T) {
	const river = "Name:Test River\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U | ConditionCheckSVar$ Dropped | ConditionSVarCompare$ EQ0 | SubAbility$ Alt | SpellDescription$ Blue, or black after a land drop.\n" +
		"SVar:Alt:DB$ Mana | Produced$ B | ConditionCheckSVar$ Dropped | ConditionSVarCompare$ GE1\n" +
		"SVar:Dropped:PlayerCountPropertyYou$LandsPlayed\n"
	const gem = "Name:Test Gem\nManaCost:no cost\nTypes:Legendary Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | ConditionCheckSVar$ Lucky | ConditionSVarCompare$ EQ0 | SubAbility$ Colour | SpellDescription$ Colourless, or any colour with a luck counter.\n" +
		"SVar:Colour:DB$ Mana | Produced$ Any | ConditionCheckSVar$ Lucky | ConditionSVarCompare$ GE1\n" +
		"SVar:Lucky:Count$CardCounters.LUCK\n"
	mana := func(d *kop.CardDef) (n int, ab *kop.Ability) {
		for i := range d.Abilities {
			if d.Abilities[i].Kind == kop.AbMana {
				n++
				ab = &d.Abilities[i]
			}
		}
		return n, ab
	}
	for _, c := range []struct {
		name, src     string
		root          kop.ColorMask
		gate          uint8
		ctr           kop.CounterKind
		arm           kop.ColorMask
		wantNoProgram bool
	}{
		{"Test River", river, kop.ColU, kop.SplitGateLands, 0, kop.ColB, true},
		{"Test Gem", gem, kop.ColC, kop.SplitGateCounter, kop.CtrLuck, kop.ColAny, true},
	} {
		def, rs := kwardpowBuild(t, c.name, strings.Replace(c.src, "Test River", c.name, 1))
		if len(rs) != 0 {
			t.Errorf("%s: refused %v", c.name, rs)
			continue
		}
		n, ab := mana(&def)
		if n != 1 || ab.ManaColors != c.root || ab.ManaN != 1 || ab.SplitGate != c.gate || ab.SplitCtr != c.ctr || ab.SplitCols != c.arm || len(ab.Prog) != 0 {
			t.Errorf("%s: %d mana abilities, %+v; want one: root %#x, gate %d, ctr %d, arm %#x, no program", c.name, n, ab, c.root, c.gate, c.ctr, c.arm)
		}
	}
	// a plain mana ability carries no arm
	def, rs := kwardpowBuild(t, "Test Plain", "Name:Test Plain\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ Add {U}.\n")
	if _, ab := mana(&def); len(rs) != 0 || ab.SplitGate != kop.SplitGateNone || ab.SplitCols != 0 {
		t.Errorf("plain mana ability: %+v, refusals %v", ab, rs)
	}
	for _, c := range []struct{ name, src, want string }{
		{"gate compare", strings.Replace(river, "GE1", "GE2", 1), "gate compare"},
		{"root compare", strings.Replace(river, "EQ0", "EQ1", 1), "gate compare"},
		{"unknown gate SVar", strings.Replace(river, "PlayerCountPropertyYou$LandsPlayed", "Count$YourLifeTotal", 1), "gate SVar"},
		{"unknown counter", strings.Replace(gem, "LUCK", "NOSUCHCTR", 1), "gate counter"},
		{"two arms", strings.Replace(river, "ConditionSVarCompare$ GE1", "ConditionSVarCompare$ GE1 | SubAbility$ Alt2", 1) + "SVar:Alt2:DB$ Mana | Produced$ R\n", "no lone DB$ Mana sub"},
		{"arm amount", strings.Replace(river, "Produced$ B |", "Produced$ B | Amount$ 2 |", 1), "sub param Amount$"},
		{"arm combo", strings.Replace(river, "Produced$ B |", "Produced$ Combo W U |", 1), "sub Produced$"},
		{"root fee", strings.Replace(river, "Cost$ T", "Cost$ 1 T", 1), "root cost beyond {T}"},
		{"root two mana", strings.Replace(river, "Produced$ U", "Produced$ U U", 1), "root Produced$"},
		{"root restricted", strings.Replace(river, "Produced$ U |", "Produced$ U | RestrictValid$ Spell.Creature |", 1), "root param RestrictValid$"},
		{"different SVars", strings.Replace(river, "ConditionCheckSVar$ Dropped | ConditionSVarCompare$ GE1", "ConditionCheckSVar$ Other | ConditionSVarCompare$ GE1", 1) + "SVar:Other:PlayerCountPropertyYou$LandsPlayed\n", "different SVars"},
	} {
		_, rs := kwardpowBuild(t, "Test River", strings.Replace(c.src, "Test Gem", "Test River", 1))
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		if len(rs) == 0 || !strings.Contains(strings.Join(all, "; "), c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}
