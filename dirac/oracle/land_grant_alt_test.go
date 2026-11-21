package oracle

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A land under Stormtide Leviathan is an Island, so it has the intrinsic
// "{T}: Add {U}" beside its own ability (CR 305.6). A bounce land's
// "{T}: Add {B}{G}" and Temple of the False God's "{T}: Add {C}{C}" (five or
// more lands) are then one permanent with two productions, the card's own
// first and the intrinsic one after (gorge appendAvailableManaAbilitiesGate).
// gorge's dedup keys on the ability's raw Produced$, so even Simic Growth
// Chamber ("G U") keeps the intrinsic {U}. The kernel's own-vs-intrinsic pair
// unit (static_landgrantalt.go) models it: the planner's pair of unit types
// with the intrinsic ability the smaller key 8 step, and the census's two
// faces per land. Every pod below holds the Leviathan, bounce lands, the Temple
// and spells those lands pay.
var landGrantAltShared = []string{
	"Stormtide Leviathan", "Stormtide Leviathan",
	"Azorius Chancery", "Simic Growth Chamber", "Golgari Rot Farm", "Selesnya Sanctuary", "Orzhov Basilica",
	"Temple of the False God", "Sol Ring", "Arcane Signet",
	"Island", "Island", "Island", "Island", "Plains", "Plains", "Plains", "Swamp", "Swamp", "Swamp", "Forest", "Forest", "Forest",
	"Wall of Omens", "Llanowar Elves", "Elvish Mystic", "Birds of Paradise", "Coiling Oracle", "Kodama's Reach", "Cultivate",
	"Rampant Growth", "Counterspell", "Beast Within", "Gray Merchant of Asphodel", "Sakura-Tribe Elder", "Mulldrifter",
}

// landGrantAltRun plays the seeds of the pod and counts the decision points
// where a Leviathan and an untapped bounce land or Temple (with five lands)
// coexist: gorge then lists both productions of that land.
func landGrantAltRun(t *testing.T, seeds uint64) (reached, bad int) {
	t.Helper()
	set := edhSet(t)
	check := func(g *GorgeSide, _ Side) *Divergence {
		var lev bool
		var src int
		for p := range 4 {
			lands := 0
			ids := g.E.G.Zone(state.ZBattlefield, state.PlayerID(p))
			for _, id := range ids {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
					if o.Face().Name == "Stormtide Leviathan" {
						lev = true
					}
					if slices.Contains(o.Face().Types, "Land") {
						lands++
					}
				}
			}
			for _, id := range ids {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && !o.Tapped {
					switch o.Face().Name {
					case "Azorius Chancery", "Simic Growth Chamber", "Golgari Rot Farm", "Selesnya Sanctuary", "Orzhov Basilica":
						src++
					case "Temple of the False God":
						if lands >= 5 {
							src++
						}
					}
				}
			}
		}
		if lev && src > 0 {
			reached++
		}
		return nil
	}
	pod := [4][]string{landGrantAltShared, landGrantAltShared, landGrantAltShared, landGrantAltShared}
	play := fourSeatSetPlay(t, set, pod, nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			bad++
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
		if res.StallUnmodelled || strings.Contains(res.Stall, "landgrant") {
			bad++
			t.Errorf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	return reached, bad
}

// TestFourSeatLandGrantAltLockstep: the pod builds (no landgrant refusal), no
// seed diverges or stalls, the Leviathan meets an untapped two-production
// land, and the planner takes both faces of such lands.
func TestFourSeatLandGrantAltLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantAltRun(t, 24)
	if reached == 0 {
		t.Fatal("the Leviathan never met an untapped bounce land or Temple: the pair is unexercised")
	}
	own, intr := dirac.BlgStats.AltOwn.Load(), dirac.BlgStats.AltIntr.Load()
	if own == 0 || intr == 0 {
		t.Fatalf("the planner tapped the pair's own face %d times and its intrinsic face %d times: both productions must be taken", own, intr)
	}
	t.Logf("%d decision points with the Leviathan beside an untapped two-production land; planner taps: own face %d, intrinsic %d", reached, own, intr)
}

// landGrantAltMutated plays the pod under a mutation and reports how many of
// the seeds diverged or stalled.
func landGrantAltMutated(t *testing.T, seeds uint64) int {
	t.Helper()
	set := edhSet(t)
	pod := [4][]string{landGrantAltShared, landGrantAltShared, landGrantAltShared, landGrantAltShared}
	play := fourSeatSetPlay(t, set, pod, nil, nil, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	bad := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		if res := play(seed); res.Div != nil || res.StallUnmodelled {
			bad++
		}
	}
	return bad
}

// TestLandGrantAltPlanMutation: with the planner seeing a land's own face alone
// the pod diverges: gorge pays through the intrinsic face (a bounce land's {U}
// for a {U} pip, its two mana kept for the next cast), so the planner's pair
// is what the lockstep holds.
func TestLandGrantAltPlanMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantAltPlan = true
	defer func() { dirac.DebugLandGrantAltPlan = false }()
	if bad := landGrantAltMutated(t, 60); bad == 0 {
		t.Fatal("no seed diverged with the planner seeing the own face alone")
	}
}

// TestLandGrantAltAvailMutation: with the census counting a land's own face
// alone, gorge's offers (a play paid through the intrinsic face) differ.
func TestLandGrantAltAvailMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantAltAvail = true
	defer func() { dirac.DebugLandGrantAltAvail = false }()
	if bad := landGrantAltMutated(t, 60); bad == 0 {
		t.Fatal("no seed diverged with the census counting the own face alone")
	}
}

// TestLandGrantAltOffRefused: with the alternative view off the pod is refused
// at its start, naming the Leviathan and the lands the view covers.
func TestLandGrantAltOffRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantAltOff = true
	defer func() { dirac.DebugLandGrantAltOff = false }()
	set := edhSet(t)
	var lists [][]cardset.Entry
	for range 4 {
		lists = append(lists, landGrantEntries(landGrantAltShared))
	}
	_, _, err := set.MatchupN(lists)
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != "landgrant" {
		t.Fatalf("the pod without the alternative view: %v, want the landgrant refusal", err)
	}
	for _, n := range []string{"Stormtide Leviathan", "Golgari Rot Farm", "Temple of the False God"} {
		if !slices.Contains(ir.Cards, n) {
			t.Errorf("refusal names %v, want %s among them", ir.Cards, n)
		}
	}
}

// TestLandGrantAltPainStillRefused: the refusal is card-precise. A painland
// (Sungrass Prairie) beside the Leviathan is still refused, naming it and the
// Leviathan, and not the bounce lands or the Temple the pair covers.
func TestLandGrantAltPainStillRefused(t *testing.T) {
	set := edhSet(t)
	deck := append(slices.Clone(landGrantAltShared), "Sungrass Prairie")
	var lists [][]cardset.Entry
	for range 4 {
		lists = append(lists, landGrantEntries(deck))
	}
	_, _, err := set.MatchupN(lists)
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != "landgrant" {
		t.Fatalf("the pod with a painland: %v, want the landgrant refusal", err)
	}
	for _, n := range []string{"Stormtide Leviathan", "Sungrass Prairie"} {
		if !slices.Contains(ir.Cards, n) {
			t.Errorf("refusal names %v, want %s among them", ir.Cards, n)
		}
	}
	for _, n := range []string{"Golgari Rot Farm", "Temple of the False God", "Azorius Chancery"} {
		if slices.Contains(ir.Cards, n) {
			t.Errorf("refusal names %s (%v): the pair covers it", n, ir.Cards)
		}
	}
}
