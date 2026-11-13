package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// reasons is rs' refusal texts.
func reasons(rs []refusal) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Reason)
	}
	return out
}

// hussarScript is Court Hussar's enter pair: the dig trigger and the
// "sacrifice it unless {W} was spent to cast it" one (CR 601.2h), whose
// ManaNotSpent$ clause this file's lowering reads. gorge reads the script;
// the kernel reads the CardDef kgen lowers it to.
const hussarScript = "Name:Spend Gate Probe\nManaCost:2 U\nTypes:Creature Vedalken Knight\nPT:1/3\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDig | TriggerDescription$ When CARDNAME enters, look at the top three cards of your library, then put one of them into your hand and the rest on the bottom of your library in any order.\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSac | ManaNotSpent$ W | TriggerDescription$ When CARDNAME enters, sacrifice it unless {W} was spent to cast it.\n" +
	"SVar:TrigDig:DB$ Dig | DigNum$ 3 | Defined$ You\n" +
	"SVar:TrigSac:DB$ Sacrifice\n" +
	"Oracle:x\n"

// viperScript is Patagia Viper's enter pair with ManaNotSpent$ U.
const viperScript = "Name:Spend Gate Viper\nManaCost:3 G\nTypes:Creature Snake\nPT:2/1\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSac | ManaNotSpent$ U | TriggerDescription$ When CARDNAME enters, sacrifice it unless {U} was spent to cast it.\n" +
	"SVar:TrigSac:DB$ Sacrifice\n" +
	"Oracle:x\n"

// TestTriggerManaSpendCond: a trigger's ManaSpent$ / ManaNotSpent$ colour
// lowers to the self-ETB Adamant gate the kernel binds as the trigger queues
// (kw_evoke.go): the colour's count read as it fires and from the bound
// count as it resolves. The gate arms pick the polarity.
func TestTriggerManaSpendCond(t *testing.T) {
	for _, c := range []struct {
		name, script string
		colour       int
		spent, unsp  int64
	}{
		{"hussar", hussarScript, 0, 0, 1}, // W, fires unless spent
		{"viper", viperScript, 1, 0, 1},   // U, fires unless spent
	} {
		t.Run(c.name, func(t *testing.T) {
			def, pt, refs := kdiscfiltBuild(t, c.name, c.script)
			if len(refs) != 0 {
				t.Fatalf("refusals: %q", refs)
			}
			found := 0
			for i := range def.Abilities {
				a := &def.Abilities[i]
				if a.Trig.Cond == 0 {
					continue
				}
				found++
				cond := pt.conds[a.Trig.Cond]
				if cond.Kind != kop.CondValue || cond.Neg {
					t.Fatalf("cond %v, want a plain value gate", cond)
				}
				if cond.Op != kop.CmpEQ {
					t.Fatalf("op %v, want EQ 1", cond.Op)
				}
				v := pt.exprs[cond.A.N]
				if v.Kind != kop.ExAdamant || int(v.N) != c.colour|1<<3 {
					t.Fatalf("value %+v, want ExAdamant colour %d count 1", v, c.colour)
				}
				if v.Sub[0].K != kop.VConst || int64(v.Sub[0].N) != c.spent {
					t.Fatalf("spent arm %+v, want const %d", v.Sub[0], c.spent)
				}
				if v.Sub[1].K != kop.VConst || int64(v.Sub[1].N) != c.unsp {
					t.Fatalf("unspent arm %+v, want const %d", v.Sub[1], c.unsp)
				}
			}
			if found != 1 {
				t.Fatalf("%d gated triggers, want 1", found)
			}
		})
	}
}

// TestTriggerManaSpendRefusals: the shapes outside the lowered class refuse
// by name, never by lowering a half-read gate.
func TestTriggerManaSpendRefusals(t *testing.T) {
	for _, c := range []struct{ name, trig, want string }{
		{"colour list", "T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSac | ManaNotSpent$ W U", "trigger ManaSpend colours"},
		{"colourless", "T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSac | ManaNotSpent$ C", "trigger ManaSpend colours"},
		{"not the card's own ETB", "T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigSac | ManaNotSpent$ W", "outside the card's own ETB trigger"},
		{"other creatures entering", "T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature | Execute$ TrigSac | ManaNotSpent$ W", "outside the card's own ETB trigger"},
		{"ManaSpent polarity", "T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDestroy | ManaSpent$ R", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			body := "SVar:TrigSac:DB$ Sacrifice\nSVar:TrigDestroy:DB$ Destroy | ValidTgts$ Artifact\n"
			def, pt, refs := kdiscfiltBuild(t, c.name,
				"Name:Spend Refusal\nManaCost:R\nTypes:Creature Goblin\nPT:2/2\n"+c.trig+"\n"+body+"Oracle:x\n")
			if c.want == "" {
				// ManaSpent lowers: the cond is the spent arm polarity.
				var cond *kop.Cond
				for i := range def.Abilities {
					if def.Abilities[i].Trig.Cond != 0 {
						k := pt.conds[def.Abilities[i].Trig.Cond]
						cond = &k
					}
				}
				if cond == nil || cond.Kind != kop.CondValue {
					t.Fatalf("ManaSpent did not lower a value gate: %v / %q", cond, refs)
				}
				v := pt.exprs[cond.A.N]
				if v.Kind != kop.ExAdamant || v.Sub[0].K != kop.VConst || int64(v.Sub[0].N) != 1 || int64(v.Sub[1].N) != 0 {
					t.Fatalf("ManaSpent arms wrong: %+v", v)
				}
				return
			}
			kdefWant(t, reasons(refs), c.want)
		})
	}
}
