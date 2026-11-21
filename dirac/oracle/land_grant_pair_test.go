package oracle

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

const landGrantPairRepro = "land_grant_pair"

const landGrantPairWhy = "a granted basic land type beside a mana source whose own ability is not a plain {T}: gorge offers both productions"

func landGrantEntries(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}

// TestLandGrantPairRefused: Yavimaya, Cradle of Growth (every land is a Forest)
// beside City of Traitors ({T}: Add {C}{C}) and Ferrous Lake is refused when
// the matchup is built, naming Ferrous Lake: City of Traitors is the own-vs-
// intrinsic pair the kernel models (static_landgrantalt.go) and no refusal. gorge appends one intrinsic "{T}: Add {G}" beside the
// land's own ability (CR 305.6), so a granted Forest offers both productions
// and the kernel's one-ability view stalled mid-game (evloop tail-11 cycle
// c000, landgrant after 113 decisions). The refusal names the granter and the
// non-plain land.
func TestLandGrantPairRefused(t *testing.T) {
	corpusDirOrSkip(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", landGrantPairRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(reproSet(t, r.Pool, r))
	_, _, _, err = set.Matchup(landGrantEntries(r.Cards[0]), landGrantEntries(r.Cards[1]))
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) {
		t.Fatalf("the recorded pair builds (err %v), want an InteractionRefusal", err)
	}
	if ir.Reason != "landgrant" || ir.Why != landGrantPairWhy {
		t.Fatalf("refused %q (%s), want landgrant (%s)", ir.Reason, ir.Why, landGrantPairWhy)
	}
	for _, n := range []string{"Yavimaya, Cradle of Growth", "Ferrous Lake"} {
		if !slices.Contains(ir.Cards, n) {
			t.Errorf("refusal names %v, want %s among them", ir.Cards, n)
		}
	}
	if slices.Contains(ir.Cards, "City of Traitors") {
		t.Errorf("refusal names City of Traitors (%v): a {C}{C} land beside one granted colour is the modelled pair", ir.Cards)
	}
	// the same decks without the non-plain lands build: Yavimaya beside plain
	// lands is not refused
	plain := func(names []string) []string {
		return slices.DeleteFunc(slices.Clone(names), func(n string) bool { return n == "City of Traitors" || n == "Ferrous Lake" })
	}
	if _, _, _, err := set.Matchup(landGrantEntries(plain(r.Cards[0])), landGrantEntries(plain(r.Cards[1]))); err != nil {
		t.Fatalf("Yavimaya beside plain lands is refused: %v", err)
	}
}

var landGrantPlainDeck = []string{
	"Voice of Law", "Proud Mentor", "Silver Seraph", "Arctic Aven", "Caller of Gales", "Cerulean Drake",
	"Treetop Recluse", "Wirewood Hivemaster", "Stormrider Spirit", "Skyshroud Condor",
	"Plains", "Plains", "Plains", "Island", "Island", "Island", "Forest", "Forest", "Forest", "Mountain", "Mountain",
}

// TestLandGrantFourSeatPlainLands plays four-seat pods on tail-11 with
// Yavimaya in two seats and only plain {T} lands beside it: the pod builds,
// no seed diverges or stalls, and Yavimaya reaches the battlefield.
func TestLandGrantFourSeatPlainLands(t *testing.T) {
	if isolate(t) {
		return
	}
	withYav := append(slices.Clone(landGrantPlainDeck), "Yavimaya, Cradle of Growth")
	reached := 0
	check := func(g *GorgeSide, _ Side) *Divergence {
		for p := range 4 {
			for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Yavimaya, Cradle of Growth" {
					reached++
				}
			}
		}
		return nil
	}
	set := cardsettest.Synth(t, "tail-11")
	play := fourSeatSetPlay(t, set, [4][]string{withYav, landGrantPlainDeck, withYav, landGrantPlainDeck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		res := play(seed)
		if res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	if reached == 0 {
		t.Fatal("Yavimaya never reached the battlefield: the grant is unexercised")
	}
	t.Logf("%d decision points with Yavimaya on the battlefield", reached)
}

// TestLandGrantPairMutation turns the refusal off: the recorded game then
// builds and stalls landgrant when the grant meets the non-plain land, so the
// guard is what keeps the pair out.
func TestLandGrantPairMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugLandGrantNoPairGuard
	dirac.DebugLandGrantNoPairGuard = true
	defer func() { dirac.DebugLandGrantNoPairGuard = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", landGrantPairRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if !strings.HasPrefix(res.Stall, KernelStallPrefix) || !strings.Contains(res.Stall, "landgrant") {
		t.Fatalf("with the guard off the replay ends %q (div %v), want a landgrant kernel stall", res.Stall, res.Div)
	}
}

// landGrantReplay is the first statement of a test that replays a recorded game
// of a pair the land-grant refusal now keeps out of a deal (a granter beside a
// land whose own ability is no plain {T}: the game never met the stall, or
// recorded it as declared). It runs the test alone in a child process
// (isolate) with the refusal off, as TestLandGrantPairMutation does, so the
// recording replays whole and no other test in the process sees the switch. It
// returns true in the parent, which has nothing left to do.
func landGrantReplay(t *testing.T) bool {
	t.Helper()
	if isolate(t) {
		return true
	}
	old := dirac.DebugLandGrantNoPairGuard
	dirac.DebugLandGrantNoPairGuard = true
	t.Cleanup(func() { dirac.DebugLandGrantNoPairGuard = old })
	return false
}

// landGrantRecordings are the recordings (by file name) whose pair the land-grant
// refusal keeps out of a deal now. The manifest and the glob test replay them
// through landGrantRecordingReplay, each alone with the refusal off, as the
// one-line wrappers they replaced did through landGrantReplay.
var landGrantRecordings = map[string]bool{
	"barbarian_ring_swamp_grant_over_a_pain_land_unmodelled": true,
	"combo_land_granted_type_census_order":                   true,
	"free_cast_tax_window":                                   true,
	"free_mayplay_tax_paid_by_source":                        true,
	"ghostfire_equip_target_menu":                            true,
	"mayplay_free_raise_tap_artifact":                        true,
	"storage_battery_activation":                             true,
}

// landGrantRecordingReplay is the first statement of a subtest that replays the
// recording name: a recording in landGrantRecordings runs alone with the
// refusal off (landGrantReplay, which returns true in the parent), any other
// runs in parallel (it calls t.Parallel itself, so a caller does not).
func landGrantRecordingReplay(t *testing.T, name string) bool {
	t.Helper()
	if landGrantRecordings[name] {
		return landGrantReplay(t)
	}
	t.Parallel()
	return false
}

// landGrantRefusedCards are the refused entries whose pairs the land-grant
// refusal keeps out of a deal now (a granter beside a land or creature whose own
// mana ability is no plain {T}): TestRefusedEntriesStillDiverge replays them
// with landGrantReplay.
var landGrantRefusedCards = map[string]bool{
	"Barbarian Ring": true, "Blanket of Night": true, "Dryad of the Ilysian Grove": true,
	"Nylea's Presence": true, "Thermopod": true,
}
