package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// stateCtrProbeScript is an inline stand-in for Mazemind Tome's script (the
// real one is Forge's, GPL, and never committed): the Mode$ Always
// counter-threshold state trigger and its exile-self/remember/gain-life
// body. ISPCOND is the IsPresent$ value the tests vary.
const stateCtrProbeScript = `Name:Mazemind Probe
ManaCost:2
Types:Artifact
T:Mode$ Always | IsPresent$ ISPCOND | TriggerZones$ Battlefield | Execute$ PrCtrChange | TriggerDescription$ probe
SVar:PrCtrChange:DB$ ChangeZone | Defined$ Self | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ PrCtrLife
SVar:PrCtrLife:DB$ GainLife | LifeAmount$ 4 | ConditionDefined$ Remembered | ConditionPresent$ Card.Self | ConditionCompare$ GE1
Oracle:probe
`

// stateCtrProbe lowers the inline script with ISPCOND replaced by isp and
// applying edit, returning the refusal reasons and the lowered card.
func stateCtrProbe(t *testing.T, isp string, edit func(string) string) (refusals []string, lc *lCard) {
	t.Helper()
	src := strings.ReplaceAll(stateCtrProbeScript, "ISPCOND", isp)
	if edit != nil {
		src = edit(src)
	}
	card, diags := cards.ParseBytes("mazemind_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{"Mazemind Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc = lowerCard(p, "Mazemind Probe", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	return refusals, lc
}

// TestStateCounterThresholdArm: the shape lowers to a self CounterAdded
// trigger named CheckOnTriggerOnly with the state-trigger latch bit.
func TestStateCounterThresholdArm(t *testing.T) {
	refusals, lc := stateCtrProbe(t, "Card.Self+counters_GE4_PAGE", nil)
	if len(refusals) != 0 {
		t.Fatalf("GE4 refused: %v", refusals)
	}
	var tr *lTrigger
	for _, a := range lc.Abilities {
		if a.Trigger != nil && a.Trigger.Outstanding {
			tr = a.Trigger
		}
	}
	if tr == nil {
		t.Fatal("no Outstanding trigger lowered")
	}
	if tr.Event != "CounterAdded" || tr.Step != "*" {
		t.Fatalf("event/step = %q/%q, want CounterAdded/*", tr.Event, tr.Step)
	}
	if tr.F == nil {
		t.Fatal("trigger has no self filter")
	}
}

// TestStateCounterThresholdBounds: kop.PredCountersGE packs the threshold in
// the high byte of an int16 PredN, so the arm must take 1..127 and refuse
// everything else instead of lowering a wrapped predicate.
func TestStateCounterThresholdBounds(t *testing.T) {
	refusals, _ := stateCtrProbe(t, "Card.Self+counters_GE127_PAGE", nil)
	if len(refusals) != 0 {
		t.Fatalf("GE127 refused: %v", refusals)
	}
	for _, isp := range []string{
		"Card.Self+counters_GE128_PAGE",
		"Card.Self+counters_GE255_PAGE",
		"Card.Self+counters_GE0_PAGE",
		"Card.Self+counters_LE4_PAGE",
		"Card.Self+counters_GE4_NOSUCH",
	} {
		refusals, _ := stateCtrProbe(t, isp, nil)
		if len(refusals) == 0 {
			t.Fatalf("%s accepted, want a refusal", isp)
		}
		t.Logf("%s -> %s", isp, refusals[0])
	}
}

// TestCounterFilterThresholdBounds: the generic filter path funnels the same
// PredCountersGE packing through clause's GE arm, so a threshold above 127
// must be refused there too instead of lowering a wrapped, always-true
// predicate (the Always state-trigger arm has its own bound and never gets
// here).
func TestCounterFilterThresholdBounds(t *testing.T) {
	page, err := counterKind("PAGE")
	if err != nil {
		t.Fatal(err)
	}
	build := func(thr string) ([]string, *poolTables) {
		src := strings.Join([]string{
			"Name:Counter Bound Probe",
			"ManaCost:1",
			"Types:Artifact",
			"S:Mode$ Continuous | Affected$ Creature.YouCtrl+counters_" + thr + "_PAGE | AddKeyword$ Flying | Description$ probe",
		}, "\n")
		_, pt, rs := kdiscfiltBuild(t, "Counter Bound Probe", src)
		var reasons []string
		for _, r := range rs {
			reasons = append(reasons, r.Reason)
		}
		return reasons, pt
	}
	// Control: 127 is representable and is really compared (precondition).
	rs, pt := build("GE127")
	if len(rs) != 0 {
		t.Fatalf("GE127 refused: %v", rs)
	}
	found := false
	for _, f := range pt.filters {
		if f.Pred == kop.PredCountersGE {
			if f.PredN != int16(127<<8|int(page)) {
				t.Fatalf("GE127 PredN = %d, want %d", f.PredN, int16(127<<8|int(page)))
			}
			found = true
		}
	}
	if !found {
		t.Fatal("GE127 interned no PredCountersGE filter: the static was not lowered")
	}
	// 128 and above cannot be packed (int16 high byte): refuse, never wrap.
	for _, thr := range []string{"GE128", "GE200", "GE255", "GE1000"} {
		rs, pt := build(thr)
		if len(rs) == 0 {
			t.Fatalf("%s accepted, want a refusal", thr)
		}
		if !strings.Contains(rs[0], "counter compare") {
			t.Fatalf("%s refusal = %q, want the counter compare refusal", thr, rs[0])
		}
		for _, f := range pt.filters {
			if f.Pred == kop.PredCountersGE && f.PredN < 0 {
				t.Fatalf("%s interned a wrapped PredCountersGE (PredN=%d)", thr, f.PredN)
			}
		}
	}
}

// TestStateCounterArmRefusals: every Always shape outside the lowered one
// keeps a refusal, and the unlisted-parameter refusal names the sorted-first
// key on every run (the map's iteration order must not reach it).
func TestStateCounterArmRefusals(t *testing.T) {
	refusals, _ := stateCtrProbe(t, "Card.Self+counters_GE4_PAGE", func(s string) string {
		return strings.Replace(s, "TriggerZones$ Battlefield", "TriggerZones$ Hand", 1)
	})
	if len(refusals) == 0 || !strings.Contains(refusals[0], "Always TriggerZones") {
		t.Fatalf("Hand TriggerZones refusals = %v, want the arm's zones refusal", refusals)
	}
	edit := func(s string) string {
		return strings.Replace(s, "Execute$ PrCtrChange", "Execute$ PrCtrChange | CheckSVar$ X | PresentCompare$ GE1", 1)
	}
	var first string
	for i := 0; i < 64; i++ {
		refusals, _ := stateCtrProbe(t, "Card.Self+counters_GE4_PAGE", edit)
		if len(refusals) == 0 {
			t.Fatalf("run %d: unlisted parameters accepted", i)
		}
		if i == 0 {
			first = refusals[0]
			if !strings.Contains(first, "Always CheckSVar$") {
				t.Fatalf("first unlisted key = %q, want the sorted-first CheckSVar$", first)
			}
			continue
		}
		if refusals[0] != first {
			t.Fatalf("run %d refusal = %q, first run %q", i, refusals[0], first)
		}
	}
}
