package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMysticRemoraCumulativeUpkeep pins gorge's play of the corpus Mystic
// Remora's cumulative upkeep {1} against CR 702.24a: each of its controller's
// upkeeps puts one age counter on it, then the controller pays {1} for EACH
// age counter on it or sacrifices it. Three upkeeps: the first costs {1}, the
// second {2} (two lands tapped, no more), and declining the third sacrifices
// the Remora with three age counters on it (dirac lowers this keyword to
// ExtCumulative; dirac/oracle/mysticremora_test.go plays it in lockstep).
func TestMysticRemoraCumulativeUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	remora, ok := reg.Lookup("Mystic Remora")
	if !ok {
		t.Fatal("corpus fixture: Mystic Remora missing")
	}
	e := handEngine(t)
	id := onBoardCard(t, e, 0, remora)
	var lands []state.ObjID
	for i := 0; i < 4; i++ {
		lands = append(lands, onBoardCard(t, e, 0, card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")))
	}
	tapped := func() int {
		n := 0
		for _, l := range lands {
			if e.G.Obj(l).Tapped {
				n++
			}
		}
		return n
	}
	pay := func(d *decision.Decision) int {
		for _, o := range d.Options {
			if o.Kind == "cumulative_pay" {
				return o.Index
			}
		}
		t.Fatalf("no cumulative_pay in %+v", d.Options)
		return -1
	}
	for upkeep := int32(1); upkeep <= 2; upkeep++ {
		for _, l := range lands {
			e.G.Obj(l).Tapped = false
		}
		resolveUpkeepCumulative(t, e)
		if got := e.G.Obj(id).Counter("AGE"); got != upkeep {
			t.Fatalf("upkeep %d: %d age counters, want %d", upkeep, got, upkeep)
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || optionKinds(d)["cumulative_sac"] != 1 {
			t.Fatalf("upkeep %d: no pay-or-sacrifice ask: %+v", upkeep, d)
		}
		submitChoices(t, e, pay(d))
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Fatalf("upkeep %d: paid, yet the Remora is in %v", upkeep, o.Zone)
		}
		if got := tapped(); got != int(upkeep) {
			t.Fatalf("upkeep %d: %d lands tapped, want %d (one per age counter)", upkeep, got, upkeep)
		}
	}
	for _, l := range lands {
		e.G.Obj(l).Tapped = false
	}
	resolveUpkeepCumulative(t, e)
	if got := e.G.Obj(id).Counter("AGE"); got != 3 {
		t.Fatalf("upkeep 3: %d age counters, want 3", got)
	}
	d := e.Pending()
	if d == nil {
		t.Fatal("upkeep 3: no ask")
	}
	for _, o := range d.Options {
		if o.Kind == "cumulative_sac" {
			submitChoices(t, e, o.Index)
			break
		}
	}
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("declined: the Remora is in %v, want the graveyard", o.Zone)
	}
	if got := tapped(); got != 0 {
		t.Fatalf("declined: %d lands tapped, want 0", got)
	}
}
