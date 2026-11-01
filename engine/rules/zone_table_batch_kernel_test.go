package rules

import (
	"maps"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Organ Harvest's RepeatEach | ChangeZoneTable$ True presents the whole loop
// as ONE ChangesZoneAll batch (Simic Slaw charges once) while per-move
// ChangesZone still fires per move (Black Market charges per bear); without
// the parameter the batch-of-one reading stands.

func TestRepeatEachChangeZoneTableBatchesChangesZoneAllKernel(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, organ, slaw, market, bears := zoneTableBoard(t, reg)

	// The carrier's compiled spell SA: precondition that it really is the
	// RepeatEach carrying ChangeZoneTable$ True.
	sa := corpusSA(t, reg, "Organ Harvest", "")
	if sa.API != "RepeatEach" {
		t.Fatalf("precondition: Organ Harvest spell API = %q, want RepeatEach", sa.API)
	}
	if !strings.EqualFold(strings.TrimSpace(sa.Params["ChangeZoneTable"]), "True") {
		t.Fatalf("precondition: DBRepeat carries ChangeZoneTable = %q, want True", sa.Params["ChangeZoneTable"])
	}

	probeAfterPass(e, func() {
		effects.Resolve(e, &effects.Ctx{Source: organ, Controller: 0,
			SVars: e.G.Obj(organ).Face().SVars}, sa)
	})
	if e.Pending() == nil {
		t.Fatal("the loop body's optional sacrifice never asked")
	}
	sacrificeAllBears(t, e, bears)
	settleAfterPass(e)
	drainTriggers(t, e)

	// Precondition: the loop body really moved the three distinct bears to
	// the graveyard.
	for _, id := range bears {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("test precondition: bear %d in %v, want Graveyard (the loop body's sacrifice did not move it)", id, o)
		}
	}
	if n := chargeCount(e, market, "CHARGE"); n != len(bears) {
		t.Fatalf("Black Market charge counters = %d, want %d (Mode$ ChangesZone must fire per move)", n, len(bears))
	}
	if n := chargeCount(e, slaw, "CHARGE"); n != 1 {
		t.Fatalf("Simic Slaw charge counters = %d, want 1 (ChangeZoneTable$ True must present the whole loop as ONE ChangesZoneAll batch)", n)
	}
	replayCheck(t, e, cfg)
}

func TestRepeatEachWithoutChangeZoneTableKeepsPerMoveChangesZoneAllKernel(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _, organ, slaw, market, bears := zoneTableBoard(t, reg)

	// Delete the parameter from a COPY of the compiled SA: the registry is
	// shared by every test in the binary, and the batching test above reads
	// the very same SA.
	orig := corpusSA(t, reg, "Organ Harvest", "")
	cp := *orig
	cp.Params = maps.Clone(orig.Params)
	sa := &cp
	delete(sa.Params, "ChangeZoneTable")
	if _, present := sa.Params["ChangeZoneTable"]; present {
		t.Fatal("control precondition: ChangeZoneTable still present on the compiled SA")
	}

	probeAfterPass(e, func() {
		effects.Resolve(e, &effects.Ctx{Source: organ, Controller: 0,
			SVars: e.G.Obj(organ).Face().SVars}, sa)
	})
	if e.Pending() == nil {
		t.Fatal("the loop body's optional sacrifice never asked")
	}
	sacrificeAllBears(t, e, bears)
	settleAfterPass(e)
	drainTriggers(t, e)

	for _, id := range bears {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("control precondition: bear %d in %v, want Graveyard", id, o)
		}
	}
	if n := chargeCount(e, market, "CHARGE"); n != len(bears) {
		t.Fatalf("control: Black Market charge counters = %d, want %d", n, len(bears))
	}
	// ONE api:Sacrifice of the three answered bears is ONE simultaneous
	// departure (CR 603.2c), so the ChangesZoneAll observer charges once even
	// without ChangeZoneTable$: the parameter aggregates zone changes across
	// LOOP ITERATIONS, and a single KChoose answer is a single event either
	// way. (Until the departure bracket opened a zone batch this read
	// len(bears) only because the sacrifice itself was not batched.) The
	// per-move control above still distinguishes Mode$ ChangesZone.
	if n := chargeCount(e, slaw, "CHARGE"); n != 1 {
		t.Fatalf("control: Simic Slaw charge counters = %d, want 1 (one simultaneous sacrifice is one ChangesZoneAll event)", n)
	}
}
