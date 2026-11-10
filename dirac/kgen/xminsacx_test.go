package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kxminsacxCard lowers one corpus card (script text read from the corpus,
// never copied here) and returns its CardDef and the refusal reasons joined.
func kxminsacxCard(t *testing.T, name, dir, file string, drop ...func(string) bool) (kop.CardDef, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tableCorpus(t), "cardsfolder", dir, file))
	if err != nil {
		t.Skip("no gorge corpus: ", err)
	}
	var keep []string
	for _, l := range strings.Split(string(b), "\n") {
		if len(drop) == 0 || !drop[0](l) {
			keep = append(keep, l)
		}
	}
	d, rs := kwardpowBuild(t, name, strings.Join(keep, "\n"))
	return d, kpaylifexReasons(rs)
}

// TestXMinSacXCarriers: XMin<N> on an activated ability's own cost and
// Sac<X/Spec> on a spell's or an activated ability's own cost lower to the
// kernel's cost shape (kop.Cost.XMin; CostSac with N 0); a spell's own XMin
// (gorge folds no floor into the cast) and every other place either token
// stands refuse by name.
func TestXMinSacXCarriers(t *testing.T) {
	// Ruthless Technomancer: both shapes on one activated ability. (Its
	// Treasure token script is not in this one-card probe's pool, so the
	// enters trigger is dropped.)
	d, all := kxminsacxCard(t, "Ruthless Technomancer", "r", "ruthless_technomancer.txt", func(l string) bool {
		return strings.HasPrefix(l, "T:") || strings.HasPrefix(l, "SVar:TrigToken") || strings.HasPrefix(l, "SVar:Y:")
	})
	if all != "" {
		t.Errorf("Ruthless Technomancer: refusals %q", all)
	}
	var tech *kop.Cost
	for i := range d.Abilities {
		if d.Abilities[i].Cost.Flags&kop.CostSac != 0 {
			tech = &d.Abilities[i].Cost
		}
	}
	if tech == nil || tech.N != 0 || tech.XMin != 1 {
		t.Errorf("Ruthless Technomancer: want CostSac with N 0 and XMin 1, got %+v", tech)
	}

	// Eliminate the Competition: a sorcery's additional cost, no floor
	d, all = kxminsacxCard(t, "Eliminate the Competition", "e", "eliminate_the_competition.txt")
	if all != "" {
		t.Errorf("Eliminate the Competition refused: %q", all)
	}
	var elim *kop.Cost
	for i := range d.Abilities {
		if d.Abilities[i].Cost.Flags&kop.CostSac != 0 {
			elim = &d.Abilities[i].Cost
		}
	}
	if d.SpellCost.Flags&kop.CostSac != 0 {
		elim = &d.SpellCost
	}
	if elim == nil || elim.N != 0 || elim.XMin != 0 {
		t.Errorf("Eliminate the Competition: want CostSac with N 0 and XMin 0, got %+v", elim)
	}

	// Lair of the Hydra: the floor on a mana {X} activation
	d, all = kxminsacxCard(t, "Lair of the Hydra", "l", "lair_of_the_hydra.txt")
	if all != "" {
		t.Errorf("Lair of the Hydra refused: %q", all)
	}
	floor := 0
	for i := range d.Abilities {
		if d.Abilities[i].Cost.Mana.X != 0 {
			floor = int(d.Abilities[i].Cost.XMin)
			if d.Abilities[i].Cost.Flags != 0 {
				t.Errorf("Lair of the Hydra: flags %v on the {X} ability", d.Abilities[i].Cost.Flags)
			}
		}
	}
	if floor != 1 {
		t.Errorf("Lair of the Hydra: the {X} ability's XMin %d, want 1", floor)
	}

	// Mind Grind: XMin on a spell's own Cost$ refuses (gorge finding)
	_, all = kxminsacxCard(t, "Mind Grind", "m", "mind_grind.txt")
	if !strings.Contains(all, "xmin cost") || !strings.Contains(all, "gorge folds no floor") {
		t.Errorf("Mind Grind: refusals %q, want the spell-Cost$ xmin refusal", all)
	}

	// inline probes: each refuses with its named reason
	probes := []struct{ name, src, want string }{
		{"Subcounter Probe", "Name:Subcounter Probe\nManaCost:2\nTypes:Artifact\n" +
			"A:AB$ Draw | Cost$ XMin1 T SubCounter<X/CHARGE> | NumCards$ X\nSVar:X:Count$xPaid\n",
			"xmin cost"},
		{"Flashback Probe", "Name:Flashback Probe\nManaCost:1 U\nTypes:Sorcery\n" +
			"K:Flashback:XMin1 X U\nA:SP$ Draw | NumCards$ 1\n",
			"flashback cost"},
		{"Double Sac Probe", "Name:Double Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n" +
			"A:AB$ Draw | Cost$ B Sac<X/Creature> Sac<X/Artifact> | NumCards$ X\nSVar:X:Count$xPaid\n",
			"second sacrifice part"},
		{"Life Sac Probe", "Name:Life Sac Probe\nManaCost:1 B\nTypes:Sorcery\n" +
			"A:SP$ Draw | Cost$ 1 B PayLife<X> Sac<X/Creature> | NumCards$ X\nSVar:X:Count$xPaid\n",
			"beside a second announced-X part"},
		{"Spell Param Probe", "Name:Spell Param Probe\nManaCost:X U\nTypes:Sorcery\n" +
			"A:SP$ Draw | Cost$ X U | XMin$ 1 | NumCards$ X\nSVar:X:Count$xPaid\n",
			"unread params"},
		{"Bare XMin Probe", "Name:Bare XMin Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n" +
			"A:AB$ Draw | Cost$ XMin1 1 B | NumCards$ 1\n",
			"beside no supported announced X"},
		{"Trigger Sac Probe", "Name:Trigger Sac Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n" +
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw\n" +
			"SVar:TrigDraw:AB$ Draw | Cost$ Sac<X/Creature> | NumCards$ X\nSVar:X:Count$xPaid\n",
			"outside a spell's or activated ability's own cost"},
	}
	for _, p := range probes {
		_, rs := kwardpowBuild(t, p.name, p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}
}
