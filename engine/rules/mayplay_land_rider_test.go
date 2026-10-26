package rules

// mayplay_land_rider_test.go pins Mode$ LandPlayed's ValidSA$
// SpellAbility.MayPlaySource clause ("When they do, ..." on Share the Spoils,
// The Fourth Doctor, The Eighth Doctor, Serra Paragon): the rider follows a
// land played through a may-play grant, not every land play. The matcher used
// to ignore ValidSA$, so a plain hand land drop fired it too.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mpLandRiderPlay submits the play_land option for id from the pending
// priority decision and settles the resulting stack.
func mpLandRiderPlay(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == id {
			submitChoices(t, e, o.Index)
			mpRiderSettle(t, e)
			return
		}
	}
	t.Fatalf("no play_land option for %d: %+v", id, mpRiderLabels(e))
}

func TestShareTheSpoilsLandRiderOnlyThroughGrant(t *testing.T) {
	for _, tc := range []struct {
		name       string
		viaGrant   bool
		wantExiled int // cards the rider moves from seat 1's library to exile
	}{
		{"exiled land played through the grant exiles the top card", true, 1},
		{"hand land drop leaves the library alone", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg, ids := mpRiderGame(t, nil, nil, []string{"Share the Spoils"}, nil)
			sts := ids[0]["Share the Spoils"][0]
			mpRiderSettle(t, e)
			exile := e.G.Zone(state.ZExile, 1)
			if len(exile) != 1 || e.G.Obj(exile[0]).ExiledWith != sts {
				t.Fatalf("precondition: seat 1 exile %v, want one card exiled with Share the Spoils", exile)
			}
			granted := exile[0]
			mpRiderDrive(t, e, 4, 1, state.StepMain1)
			addManaQuiet(t, e, 1, "GG")
			e.priorityRound()
			var hand state.ObjID
			for _, id := range e.G.Zone(state.ZHand, 1) {
				if e.G.Obj(id).Face().IsLand() {
					hand = id
					break
				}
			}
			if hand == 0 || hand == granted {
				t.Fatalf("precondition: no hand land for seat 1 (hand land %d, granted %d)", hand, granted)
			}
			libBefore, exBefore := len(e.G.Zone(state.ZLibrary, 1)), len(e.G.Zone(state.ZExile, 1))
			play, want := hand, 0
			if tc.viaGrant {
				play, want = granted, -1 // the played card leaves exile, the rider adds one
			}
			mpLandRiderPlay(t, e, play)
			if z := e.G.Obj(play).Zone; z != state.ZBattlefield {
				t.Fatalf("played land zone %s, want battlefield", z)
			}
			libDelta := len(e.G.Zone(state.ZLibrary, 1)) - libBefore
			exDelta := len(e.G.Zone(state.ZExile, 1)) - exBefore
			if libDelta != -tc.wantExiled || exDelta != tc.wantExiled+want {
				t.Fatalf("library delta %d, exile delta %d; want %d and %d", libDelta, exDelta, -tc.wantExiled, tc.wantExiled+want)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestLandPlayedValidSAMayPlaySourceCensus fixes the corpus's LandPlayed
// ValidSA$ spellings: landPlayedMatches models exactly one, and fails closed
// on any other, so a new card or spelling must force a decision here.
func TestLandPlayedValidSAMayPlaySourceCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	got := map[string]string{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, tr := range f.Triggers {
				if v, ok := tr.Params["ValidSA"]; ok && tr.Mode == "LandPlayed" {
					got[f.Name] = v
				}
			}
		}
	}
	var names []string
	for n, v := range got {
		names = append(names, n)
		if v != "SpellAbility.MayPlaySource" {
			t.Errorf("%s: LandPlayed ValidSA$ %q, only SpellAbility.MayPlaySource is modelled", n, v)
		}
	}
	slices.Sort(names)
	want := []string{"Serra Paragon", "Share the Spoils", "The Eighth Doctor", "The Fourth Doctor"}
	if !slices.Equal(names, want) {
		t.Fatalf("LandPlayed triggers carrying ValidSA$ = %v, want %v", names, want)
	}
}

func TestFourthDoctorFoodOnlyForLibraryTopLand(t *testing.T) {
	foods := func(e *Engine) int {
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); strings.HasPrefix(o.Face().Name, "Food") && o.Controller == 0 {
				n++
			}
		}
		return n
	}
	for _, tc := range []struct {
		name     string
		fromTop  bool
		wantFood int
	}{
		{"hand land drop makes no Food", false, 0},
		{"historic land from the library top makes a Food", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _, _ := mpRiderGame(t, nil, nil, []string{"The Fourth Doctor"}, nil)
			driveToTurn3Main(t, e)
			var hand state.ObjID
			for _, id := range e.G.Zone(state.ZHand, 0) {
				if e.G.Obj(id).Face().IsLand() {
					hand = id
					break
				}
			}
			play := hand
			if tc.fromTop {
				citadel := corpusAlternativeCard(t, "Darksteel Citadel")
				o := e.G.AddObject(citadel, 0)
				o.Zone = state.ZLibrary
				e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{o.ID}, e.G.Zone(state.ZLibrary, 0)...))
				e.priorityRound()
				play = o.ID
				if e.G.Obj(play).Zone != state.ZLibrary || e.G.Zone(state.ZLibrary, 0)[0] != play {
					t.Fatalf("precondition: land not on top of the library")
				}
			}
			if hand == 0 || foods(e) != 0 {
				t.Fatalf("precondition: hand land %d, Food count %d", hand, foods(e))
			}
			mpLandRiderPlay(t, e, play)
			if z := e.G.Obj(play).Zone; z != state.ZBattlefield {
				t.Fatalf("played land zone %s, want battlefield", z)
			}
			if got := foods(e); got != tc.wantFood {
				t.Fatalf("Food tokens = %d, want %d", got, tc.wantFood)
			}
		})
	}
}
