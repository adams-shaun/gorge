package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// unlessAddCounter returns the lowered and built AddCounter pay instructions
// of card name's abilities.
func unlessAddCounter(t *testing.T, all []*lCard, bs []built, name string) (lInstr, kop.Instr) {
	t.Helper()
	i := keclIndex(t, all, name)
	var lit lInstr
	found := false
	for _, a := range all[i].Abilities {
		for _, in := range a.Prog {
			if in.Op == "UnlessSwitched" && in.CS == "AddCounter" {
				if found {
					t.Fatalf("%s: more than one unless AddCounter", name)
				}
				lit, found = in, true
			}
		}
	}
	if !found {
		t.Fatalf("%s: no lowered unless AddCounter", name)
	}
	var kit kop.Instr
	found = false
	for _, a := range bs[i].def.Abilities {
		for _, in := range a.Prog {
			if in.Op != kop.OpUnlessSwitched {
				continue
			}
			if found {
				t.Fatalf("%s: more than one built unless pay", name)
			}
			kit, found = in, true
			if kit.C&0xf != kop.PayTapX || !kop.IsPayAddCounterB(kit.B.N) || kit.F != 0 {
				t.Fatalf("%s: unless pay C %d B.N %d F %d, want PayTapX with a packed AddCounter B.N and no filter", name, kit.C&0xf, kit.B.N, kit.F)
			}
		}
	}
	if !found {
		t.Logf("%s lowered abilities %d built %d refusals %v", name, len(all[i].Abilities), len(bs[i].def.Abilities), lowerRefusals(all[i]))
		for _, a := range bs[i].def.Abilities {
			t.Logf("%s kind %d prog %+v", name, a.Kind, a.Prog)
		}
		t.Fatalf("%s: no built unless pay", name)
	}
	return lit, kit
}

// TestUnlessAddCounterLowers lowers the two corpus cards whose UnlessCost$
// is AddCounter<N/KIND/recipient>, refuses an unknown third field by name,
// and keeps the third field out of activation costs.
func TestUnlessAddCounterLowers(t *testing.T) {
	all, bs := keclBuild(t, []string{"Cheering Crowd", "Sanctuary Wall"})

	lit, kit := unlessAddCounter(t, all, bs, "Cheering Crowd")
	if lit.Obj2.Kind != rSelf || lit.A.N != 1 || lit.B.N != int(kop.CtrP1P1) {
		t.Errorf("Cheering Crowd: lowered %+v, want Self/1/P1P1", lit)
	}
	if ck, r := kop.PayAddCounterFrom(kit.B.N); r != kop.RefSelf || ck != kop.CtrP1P1 {
		t.Errorf("Cheering Crowd: built kind %d recipient %d, want CtrP1P1 on RefSelf", ck, r)
	}

	lit, kit = unlessAddCounter(t, all, bs, "Sanctuary Wall")
	if lit.Obj2.Kind != rTarget || lit.Obj2.N != 0 || lit.A.N != 1 || lit.B.N != int(kop.CtrStun) {
		t.Errorf("Sanctuary Wall: lowered %+v, want Target0/1/Stun", lit)
	}
	if ck, r := kop.PayAddCounterFrom(kit.B.N); r != kop.RefTarget0 || ck != kop.CtrStun {
		t.Errorf("Sanctuary Wall: built kind %d recipient %d, want CtrStun on RefTarget0", ck, r)
	}

	// An unknown recipient field stays refused and is named.
	got := keclReason(keclSynth(t, "Unless Counter Probe",
		"Name:Unless Counter Probe\nManaCost:1 B\nTypes:Sorcery\nA:SP$ GainLife | UnlessCost$ AddCounter<1/P1P1/Creature.YouCtrl> | UnlessPayer$ You | LifeAmount$ 1\n"))
	if !strings.Contains(got, "AddCounter target \"Creature.YouCtrl\"") {
		t.Errorf("unknown AddCounter recipient: refusals %q, want the field named", got)
	}

	// The third-field forms are unless costs only.
	got = keclReason(keclSynth(t, "Activation Counter Probe",
		"Name:Activation Counter Probe\nManaCost:1 B\nTypes:Creature Human\nPT:1/1\nA:AB$ GainLife | Cost$ AddCounter<1/P1P1/CARDNAME> | LifeAmount$ 1\n"))
	if !strings.Contains(got, "outside an unless cost") {
		t.Errorf("activation AddCounter recipient: refusals %q, want the unless-cost refusal", got)
	}
}
