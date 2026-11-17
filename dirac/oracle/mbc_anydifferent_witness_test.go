package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The five Firemind Vessel recordings replay clean (TestFVProjectionFlagsReprosLockstep,
// TestMBCElusiveTormentorReprosLockstep), but a recording whose game drifted off the Vessel
// would replay clean too. This replays each and reads gorge's log for the scenario the
// recording names: a Vessel tapped for its two different colours (the AnyDifferent mana
// ability resolves, so a choose ask follows its tap and two mana_add events follow that), and
// for the Insidious Mist recordings an Elusive Tormentor trigger on the stack.
func TestMBCAnyDifferentRecordingsReachTheVessel(t *testing.T) {
	t.Parallel()
	for _, w := range []struct {
		name       string
		vesselTaps int // at least this many Vessel taps that float two mana
		tormentor  int // at least this many Elusive Tormentor trigger pushes
	}{
		{"anydifferent_gap_tapped_by_prefix_search", 1, 0},
		{"anydifferent_gap_tapped_by_unless_window", 1, 0},
		{"anydifferent_gap_tapped_by_ward_payment", 1, 0},
		{"insidious_mist_pay_taps_lands_then_vessel", 1, 1},
		{"insidious_mist_unblocked_pay_offer", 1, 1},
	} {
		t.Run(w.name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", w.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			// A fresh clone has no corpus: skip like every repro test rather than
			// failing to build the custom table (fresh-clone-check runs this package).
			corpusDirOrSkip(t)
			set := globSet(t, r.Pool, r)
			var vessel, tormentor int
			rc := r.RunConfig()
			rc.Finish = func(g *GorgeSide) {
				name := func(id state.ObjID) string {
					if o := g.E.G.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
						return o.Card.Faces[0].Name
					}
					return ""
				}
				evs := g.E.L.Events
				for i, e := range evs {
					switch {
					case e.Kind == events.Tap && name(e.Obj) == "Firemind Vessel":
						adds := 0
						for _, f := range evs[i+1 : min(len(evs), i+8)] {
							if f.Kind == events.ManaAdd && f.Amount > 0 {
								adds++
							}
						}
						if adds >= 2 {
							vessel++
						}
					case e.Kind == events.TriggerPush && name(e.Obj) == "Elusive Tormentor":
						tormentor++
					}
				}
			}
			res := replayWithConfig(t, set, r, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
			}
			t.Logf("%s: vessel taps floating two mana %d, tormentor triggers %d", w.name, vessel, tormentor)
			if vessel < w.vesselTaps || tormentor < w.tormentor {
				t.Fatalf("recording drifted off its scenario: vessel taps floating two mana %d (want >= %d), Elusive Tormentor triggers %d (want >= %d)", vessel, w.vesselTaps, tormentor, w.tormentor)
			}
		})
	}
}

// replayWithConfig is replayDecodedScript on the repro's own script under a caller's RunConfig
// (the Finish hook that reads gorge's log).
func replayWithConfig(t *testing.T, set cardset.Set, r *Repro, rc RunConfig) Result {
	t.Helper()
	m, err := r.Matchup(synthReg(t))
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
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}
