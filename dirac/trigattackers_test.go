package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTriggeredAttackersGroup: gorge's Defined$ TriggeredAttackers is the
// attacker group of the DeclareAttackers event that raised the trigger (the
// queue entry's Remembered), so a body on RefTriggerAttackers acts on every
// attacker of the group, not the first one: a per-defender line (Tf2DefPlayer)
// and the batch line, a pump and a counter. The "filtered" rows give the
// line a filter only one attacker matches: the trigger amount is the matched
// count (1) but the group is still the whole declaration (CR 508.3b, 508.3d).
func TestTriggeredAttackersGroup(t *testing.T) {
	type row struct {
		name   string
		flags2 kop.TrigFlags2
		filt   bool // a filter only the artifact attacker matches
		prog   kop.Instr
		check  func(p *perm) int
		want   int
	}
	pump := kop.Instr{Op: kop.OpPumpUntilEOT, Obj: kop.RefTriggerAttackers, A: kop.Const(-1), B: kop.Const(0)}
	ctr := kop.Instr{Op: kop.OpPutCounter, Obj: kop.RefTriggerAttackers, A: kop.Const(2), C: int16(kop.CtrP1P1)}
	eot := func(p *perm) int { return int(p.eotP) }
	cnt := func(p *perm) int { return p.ctrLow(int(kop.CtrP1P1)) }
	for _, r := range []row{
		{"defender pump", kop.Tf2DefPlayer | kop.Tf2AtkOpp, false, pump, eot, -1},
		{"batch pump", 0, false, pump, eot, -1},
		{"defender counter", kop.Tf2DefPlayer | kop.Tf2AtkOpp, false, ctr, cnt, 2},
		{"batch counter", 0, false, ctr, cnt, 2},
		{"defender pump filtered", kop.Tf2DefPlayer | kop.Tf2AtkOpp, true, pump, eot, -1},
		{"batch counter filtered", 0, true, ctr, cnt, 2},
	} {
		t.Run(r.name, func(t *testing.T) {
			deck := []cardset.Entry{{Name: "Plains", Count: 20}}
			set := cardsettest.Set(t, "fdn")
			tbl, _, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature}, kop.Filter{Types: kop.TArtifact})
			crea, art := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
			f := crea
			if r.filt {
				f = art
			}
			w, bear, golem := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
			tbl.Cards = append(tbl.Cards,
				kop.CardDef{Name: "Test Herald", Types: kop.TEnchantment, Abilities: []kop.Ability{
					{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttackersDeclared, Filter: f, Flags2: r.flags2}, Prog: []kop.Instr{r.prog}}}},
				kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 4},
				kop.CardDef{Name: "Test Golem", Types: kop.TCreature | kop.TArtifact, Power: 1, Toughness: 4})
			sc := newScenTable(t, tbl, Config{NoElide: true})
			s := sc.s
			sc.perm(w, 0)
			a1, a2 := sc.perm(bear, 0), sc.perm(golem, 0)
			if !sc.k.gen.anyTriggerAttackers {
				t.Fatal("the table did not arm anyTriggerAttackers")
			}
			sc.combat(t, []Obj{a1, a2}, nil)
			for _, o := range []Obj{a1, a2} {
				if got := r.check(&s.perms[sc.slot(o)]); got != r.want {
					t.Fatalf("attacker %d: got %d, want %d (both attackers of the group)", o, got, r.want)
				}
			}
		})
	}
}
