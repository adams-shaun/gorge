package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stateCtrProbeScript is an inline stand-in for Mazemind Tome's script (the
// real one is Forge's, GPL, and never committed): the Mode$ Always
// counter-threshold state trigger and its exile-self/remember/gain-life
// body. ISPCOND is the IsPresent$ value the tests vary.
const stateCtrProbeScript = `Name:Mazemind Probe
ManaCost:2
Types:Artifact
T:Mode$ Always | TriggerZones$ Battlefield | IsPresent$ ISPCOND | Execute$ TrigChange | TriggerDescription$ probe
SVar:TrigChange:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self | RememberChanged$ True | SubAbility$ DBGainLife
SVar:DBGainLife:DB$ GainLife | ConditionDefined$ Remembered | ConditionPresent$ Card.Self | ConditionCompare$ GE1 | LifeAmount$ 4
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
		return strings.Replace(s, "Execute$ TrigChange", "Execute$ TrigChange | CheckSVar$ X | PresentCompare$ GE1", 1)
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
