package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 601.2b announces the cost, then CR 601.2g-h pays it: Eviscerator's
// Insight's {4}{B} flashback sacrifices an artifact or creature as an
// additional cost, so gorge's prefix search takes its phased fallback
// (potential_script.go scriptSearch.phased) -- the Elven Chorus granted taps,
// the four Eldrazi Spawn sacrifices (each "sacrificed", then "ceased to
// exist"), a Forest and Prismite's two {2} filters, in that order. The kernel
// now carries the same free/covered/fee order (dirac/mana_grantedoffers.go
// phasedScope), so its plan sacrifices the same four Spawn, ceases them, and
// answers the two colour asks the way gorge does; before that mirror the
// kernel kept three Spawn in the graveyard and one on the battlefield and left
// the pool colourless instead of holding the converted colour.
const flashbackPaymentRepro = "flashback_phased_prefix_pays"

// flashbackPaymentObs is what the replay saw at the recorded divergence.
type flashbackPaymentObs struct {
	checks   int
	captured bool
	// tokenIDs are the Eldrazi Spawn tokens on the board at the divergence;
	// paidIDs are the four whose cease the payment's sacrifices left last.
	tokenIDs, paidIDs []state.ObjID
	// sacrificed and ceased are the zone events the payment left per token.
	sacrificed map[state.ObjID]bool
	ceased     map[state.ObjID]bool
	// zones are both projections' zones for each token at the divergence.
	refZone, kerZone map[state.ObjID]uint8
	// pool and kernelPool are both sides' floating mana at the divergence.
	pool, kernelPool [6]int16
}

// replayFlashbackPayment replays the recorded game on the autopay surface and
// fails on any divergence, an action the recording could not serve, or a
// replay that never reached the payment. It observes the state at the
// recording's divergence (the decision after the payment).
func replayFlashbackPayment(t *testing.T) (*Repro, *flashbackPaymentObs) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", flashbackPaymentRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := globSet(t, r.Pool, r)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	obs := &flashbackPaymentObs{
		sacrificed: map[state.ObjID]bool{},
		ceased:     map[state.ObjID]bool{},
		refZone:    map[state.ObjID]uint8{},
		kerZone:    map[state.ObjID]uint8{},
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	rc.Check = func(ref *GorgeSide, k Side) *Divergence {
		at := obs.checks
		obs.checks++
		if r.Divergence == nil || at != r.Divergence.At {
			return nil
		}
		obs.captured = true
		var rp, kp Projection
		ref.Project(&rp)
		k.Project(&kp)
		obs.pool, obs.kernelPool = rp.Pool[0], kp.Pool[0]
		for _, ev := range ref.E.L.Events {
			switch {
			case events.IsSacrifice(ev):
				obs.sacrificed[ev.Obj] = true
			case events.IsCease(ev):
				obs.ceased[ev.Obj] = true
			}
		}
		hash := nameHash("Eldrazi Spawn Token")
		spawn := map[state.ObjID]bool{}
		for i := 0; i < int(rp.NInst) && i < MaxInst; i++ {
			if rp.Cards[i].NameHash != hash || rp.Cards[i].Zone == ZoneNone {
				continue
			}
			id := ref.id.obj[i]
			obs.tokenIDs = append(obs.tokenIDs, id)
			spawn[id] = true
			obs.refZone[id], obs.kerZone[id] = rp.Cards[i].Zone, kp.Cards[i].Zone
		}
		// The payment's four are the last four Spawn to cease: any earlier
		// Spawn the game sacrificed are older history.
		for _, ev := range ref.E.L.Events {
			if events.IsCease(ev) && spawn[ev.Obj] {
				obs.paidIDs = append(obs.paidIDs, ev.Obj)
			}
		}
		if n := len(obs.paidIDs); n > 4 {
			obs.paidIDs = obs.paidIDs[n-4:]
		}
		return nil
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, pol, rc)
	if pol.Skipped != 0 {
		t.Fatalf("replay skipped %d of %d recorded actions (first at decision %d): its game path has drifted",
			pol.Skipped, len(r.Actions), pol.firstSkipAt)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !obs.captured {
		t.Fatalf("the replay never reached the recorded divergence at decision %d", r.Divergence.At)
	}
	return r, obs
}

// TestFlashbackPaymentTokenCease: the flashback's additional cost sacrifices
// four Eldrazi Spawn, so each one is sacrificed (battlefield -> graveyard) and
// then ceases to exist (CR 704.5d), as gorge's log shows; the kernel's plan
// used to leave three in the graveyard and one on the battlefield.
func TestFlashbackPaymentTokenCease(t *testing.T) {
	t.Parallel()
	_, obs := replayFlashbackPayment(t)
	if len(obs.paidIDs) != 4 {
		t.Fatalf("the payment ceased %d Eldrazi Spawn tokens, want four", len(obs.paidIDs))
	}
	for _, id := range obs.paidIDs {
		if !obs.sacrificed[id] {
			t.Errorf("Eldrazi Spawn %d ceased without being sacrificed", id)
		}
		if obs.refZone[id] != ZoneCeased {
			t.Errorf("Eldrazi Spawn %d is in zone %d in gorge at the divergence, want ceased", id, obs.refZone[id])
		}
		if obs.kerZone[id] != obs.refZone[id] {
			t.Errorf("Eldrazi Spawn %d is in zone %d in the kernel, zone %d in gorge", id, obs.kerZone[id], obs.refZone[id])
		}
	}
}

// TestFlashbackPaymentConversionColour: Prismite's two "Pay 2: Add any color"
// activations ask one colour each; gorge answers W then B, the {B} pays the
// flashback, and the converted white stays in the pool. The kernel's plan used
// to keep the three colourless mana instead.
func TestFlashbackPaymentConversionColour(t *testing.T) {
	t.Parallel()
	_, obs := replayFlashbackPayment(t)
	if obs.pool[5] != 0 {
		t.Fatalf("the pool still holds %d colourless at the divergence: the conversion did not spend it", obs.pool[5])
	}
	if obs.pool[0] == 0 {
		t.Fatalf("the pool holds no white at the divergence: the first conversion's colour was not kept")
	}
	if obs.pool != obs.kernelPool {
		t.Fatalf("pool gorge %v, kernel %v at the divergence", obs.pool, obs.kernelPool)
	}
}
