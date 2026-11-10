package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Inline stand-ins for the two mass writer shapes (the real scripts are
// Forge's, GPL, and never committed); the holes are what the variants change.
const pumpAllRememberProbe = `Name:Sweep Probe
ManaCost:2 B
Types:Sorcery
A:SP$ PumpAll | ValidCards$ Creature.powerLE2 PARAMS| SubAbility$ DBKill
SVar:DBKill:DB$ DestroyAll | ValidCards$ Card.IsRemembered | SubAbility$ DBEnd
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
Oracle:probe
`

const destroyLKIProbe = `Name:Destroy Probe
ManaCost:1 W
Types:Instant
A:SP$ Destroy | ValidTgts$ Artifact PARAMS| SubAbility$ DBDraw
SVar:DBDraw:DB$ Draw | ConditionDefined$ RememberedLKI | ConditionPresent$ Card.YouCtrl | ConditionCompare$ EQ1 | SubAbility$ DBEnd
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
Oracle:probe
`

// TestRememberWritersMass: PumpAll RememberPumped$ and Destroy RememberLKI$
// (without AlwaysRemember$) are remembered-set writers, so the same-chain
// readers lower instead of looking like cross-ability reads; a chain with no
// writer still refuses by name. Day of Black Sun and Break the Spell lower
// clean; Faerie Slumber Party stays refused (its
// count reads each returned creature's last controller, which the kernel's
// one-object capture cannot hold).
func TestRememberWritersMass(t *testing.T) {
	// PumpAll: an ExtRememberAll over the pump's filter, right before the pump
	lc := lowerProbe(t, pumpAllRememberProbe, "Sweep Probe", "| RememberPumped$ True ")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("PumpAll RememberPumped$ refused: %s", r)
	}
	if ops := strings.Join(progOps(lc), " "); !strings.Contains(ops, "RememberAll PumpAll DestroyAll") {
		t.Errorf("lowered to %q, want RememberAll right before PumpAll", ops)
	}
	if got := probeRefusals(lowerProbe(t, pumpAllRememberProbe, "Sweep Probe", "")); !strings.Contains(got, "reads Remembered written by another ability") {
		t.Errorf("the same chain without a writer: refusals %q, want the cross-ability read", got)
	}
	for _, tc := range []struct{ params, want string }{
		{"| RememberPumped$ False ", `RememberPumped "False"`},
		{"| RememberPumped$ True | ValidTgts$ Player ", "PumpAll RememberPumped$ with a ValidTgts$ player scope"},
	} {
		if got := probeRefusals(lowerProbe(t, pumpAllRememberProbe, "Sweep Probe", tc.params)); !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want %q", tc.params, got, tc.want)
		}
	}

	// Destroy: the RememberDestroyed$ arm (C bit 1) on the one target
	lc = lowerProbe(t, destroyLKIProbe, "Destroy Probe", "| RememberLKI$ True ")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("Destroy RememberLKI$ refused: %s", r)
	}
	var d *lInstr
	for i, in := range lc.Abilities[0].Prog {
		if in.Op == "Destroy" {
			d = &lc.Abilities[0].Prog[i]
		}
	}
	if d == nil || d.Note != noteRememberDestroyed || d.C&2 == 0 {
		t.Fatalf("Destroy %+v, want the RememberDestroyed arm (C bit 1)", d)
	}
	if got := probeRefusals(lowerProbe(t, destroyLKIProbe, "Destroy Probe", "")); !strings.Contains(got, "reads Remembered written by another ability") {
		t.Errorf("the same chain without a writer: refusals %q, want the cross-ability read", got)
	}
	lkiRead := strings.Replace(destroyLKIProbe, "DB$ Draw |", "DB$ Draw | NumCards$ X |", 1) + "SVar:X:RememberedLKI$CardPower\n"
	for _, tc := range []struct{ script, params, want string }{
		{destroyLKIProbe, "| RememberLKI$ True | TargetMin$ 0 | TargetMax$ 2 ", `Destroy RememberLKI$ with TargetMax$ "2"`},
		{lkiRead, "| RememberLKI$ True ", "Destroy RememberLKI$ beside a remembered characteristic read"},
	} {
		if got := probeRefusals(lowerProbe(t, tc.script, "Destroy Probe", tc.params)); !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want %q", tc.params, got, tc.want)
		}
	}

	// the corpus cards
	names := []string{"Day of Black Sun", "Break the Spell", "Faerie Slumber Party"}
	lw := lowerNamedPool(t, names)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, c := range all {
		if !c.Token {
			byName[c.Name] = i
		}
	}
	for _, name := range names {
		i, ok := byName[name]
		if !ok {
			t.Fatalf("%s: not lowered", name)
		}
		r := lowerRefusals(all[i])
		if name == "Faerie Slumber Party" {
			if len(r) == 0 || !strings.Contains(probeRefusals(all[i]), "RememberLKI") {
				t.Errorf("%s: refusals %v, want its ChangeZoneAll RememberLKI$ named", name, r)
			}
			continue
		}
		if len(r) > 0 {
			t.Errorf("%s: refused: %v", name, r)
			continue
		}
		found := false
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				found = found || in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtRememberAll ||
					in.Op == kop.OpDestroy && in.C&2 != 0
			}
		}
		if !found {
			t.Errorf("%s: no remember writer in the built program", name)
		}
	}
}
