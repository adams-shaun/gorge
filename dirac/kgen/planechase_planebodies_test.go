package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// planeBodyPin is one plane's refusal pin: want is a refusal substring that
// must still appear among the card's reasons, gone one that must not (the
// shape a ticket wave moved).
type planeBodyPin struct {
	card, want, gone, form string
}

// planechasePlaneBodies pins the three corpus planes' remaining kgen
// refusals. The planebodies ticket's shapes are lowered on their ChaosEnsues
// bodies (zone_digplanar.go, api_planechase.go) and, since this ticket, on
// Selesnya Loft Gardens' two ActiveZones$ Command replacements (probe
// planes); the tapsformana wave on main already admitted the reflected
// command-zone TapsForMana triggers of Eloren Wilds and The Fertile Lands of
// Saulvinia, which lower whole now. What is left is Selesnya Loft Gardens'
// until-end-of-turn Effect trigger (effectTapExtraRefuse) and Eloren Wilds'
// DB$ Effect chaos body.
var planechasePlaneBodies = []planeBodyPin{
	// Eloren Wilds: the chaos body's DB$ Effect (a CantBeCast static over
	// the remembered player beside a PlaneswalkedTo exile trigger) is the
	// planebodies ticket's next shape; its reflected command-zone
	// TapsForMana trigger lowered on main's tapsformana wave.
	{card: "Eloren Wilds", form: "ChaosEnsues body -> Effect static+trigger", want: "Effect StaticAbilities$ \"STCantCast\" beside"},
	// The Fertile Lands of Saulvinia lowers whole: the planar dig, the
	// remembered-plane chaos, the Cleanup and its reflected command-zone
	// TapsForMana trigger (main's tapsformana wave) all moved.
	{card: "The Fertile Lands of Saulvinia", form: "planar dig body moved", gone: "DB$ DigUntil"},
	// Selesnya Loft Gardens: its Effect TapsForMana trigger keeps the
	// tap-extra ticket's refusal; the two ActiveZones$ Command replacements
	// lower on the plane face (the planebodies ticket's shape, played in
	// lockstep on synthetic probe planes).
	{card: "Selesnya Loft Gardens", form: "TapsForMana Effect trigger (tap-extra ticket)", want: "Effect TapsForMana trigger until end of turn"},
	{card: "Selesnya Loft Gardens", form: "the two ActiveZones$ Command replacements moved", gone: "repl-hard: CreateToken ActiveZones"},
	{card: "Selesnya Loft Gardens", form: "the two ActiveZones$ Command replacements moved", gone: "repl-hard: AddCounter ActiveZones"},
	// The round's plane-face net: every replacement kind but CreateToken/
	// AddCounter refuses on a plane face (the kernel's plane count models
	// only the two unscoped doublers, so any other kind -- Kessig's
	// DamageDone prevent, The Doctor's Tomb's dies-to-exile, Sea of Sand's
	// draw replacer -- would lower to a record the kernel never applies).
	{card: "Kessig", form: "DamageDone replacement on a plane face", want: "repl-hard: DamageDone replacement on a plane face"},
	{card: "The Doctor's Tomb", form: "dies-to-exile Moved replacement on a plane face", want: "repl-hard: Moved replacement on a plane face"},
	{card: "Sea of Sand", form: "Draw replacement on a plane face", want: "repl-hard: Draw replacement on a plane face"},
}

// The dig plane's chaos body with both library positions ABSENT: gorge's
// planar arm reads an absent (or "0") position as the stay-in-place top
// default, which emits no event, while the kernel's planar arm bottoms every
// placement unconditionally (zone_digplanar.go planeDeckToBottom) -- so the planar
// arm requires both positions to be the explicit "-1" the corpus carrier
// spells, and refuses the shape.
const planebodiesDigPlaneNoPos = `Name:Planebodies Dig NoPos Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ PlanebodiesDig | TriggerDescription$ chaos: dig the planar deck, erupt the found plane
SVar:PlanebodiesDig:DB$ DigUntil | Valid$ Plane | DigZone$ PlanarDeck | RememberFound$ True | FoundDestination$ PlanarDeck | RevealedDestination$ PlanarDeck | SubAbility$ PlanebodiesChaos
SVar:PlanebodiesChaos:DB$ ChaosEnsues | Defined$ Remembered | SubAbility$ PlanebodiesCleanup
SVar:PlanebodiesCleanup:DB$ Cleanup | ClearRemembered$ True
Oracle:chaos: dig the planar deck, erupt the found plane
`

// A plane face carrying an ActiveZones-less R: line (kavaron's ETB-tapped
// shape): every tolerant arm's empty-ActiveZones read would lower it, and
// gorge's per-event planar scan would collect it, so the plane-face entry
// refuses the kind by name before any arm runs.
const planebodiesMovedPlaneSrc = `Name:Planebodies Moved Probe Plane
ManaCost:no cost
Types:Plane Probe
R:Event$ Moved | ReplacementResult$ Updated | ValidCard$ Card.Self | ReplaceWith$ ETBTapped | Destination$ Battlefield | Description$ This plane enters tapped.
SVar:ETBTapped:DB$ Tap | ETB$ True | Defined$ Self
Oracle:This plane enters tapped.
`

// lowerPlaneProbe lowers one synthetic plane script and joins its refusal
// reasons.
func lowerPlaneProbe(t *testing.T, src string) string {
	t.Helper()
	card, diags := cards.ParseBytes("planebodies_probe_plane.txt", []byte(src))
	if card == nil {
		t.Fatalf("plane probe parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("plane probe link: %v", d)
	}
	name := card.Faces[0].Name
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, name, card, false)
	var rs []string
	for _, r := range lc.Refusals {
		rs = append(rs, r.Reason)
	}
	return strings.Join(rs, "; ")
}

// TestPlanechasePlanarDigPositionsAndPlaneFaceNetRefused pins the round's two
// refusal tightenings on synthetic plane scripts (the corpus carriers spell
// both "-1" positions and only the two Selesnya bodies, so no corpus plane
// exercises either):
//   - the planar dig's absent FoundLibraryPosition$/RevealedLibraryPosition$
//     (gorge stays-in-place, the kernel bottoms: the planar arm requires the
//     explicit "-1");
//   - an ActiveZones-less R: line of any kind but CreateToken/AddCounter on a
//     plane face (the tolerant arms' empty-ActiveZones read would lower a
//     record the kernel's plane count never applies).
func TestPlanechasePlanarDigPositionsAndPlaneFaceNetRefused(t *testing.T) {
	if got := lowerPlaneProbe(t, planebodiesDigPlaneNoPos); !strings.Contains(got, "DigUntil planar FoundLibraryPosition$") {
		t.Errorf("the absent-position planar dig lowered: refusals %q", got)
	}
	if got := lowerPlaneProbe(t, planebodiesMovedPlaneSrc); !strings.Contains(got, "repl-hard: Moved replacement on a plane face") {
		t.Errorf("the ActiveZones-less Moved plane-face line lowered: refusals %q", got)
	}
}

// TestPlanechasePlaneBodiesRefused pins the three corpus planes' remaining
// refusals: every shape the tapsformana ticket owns must still refuse by
// name, and every ChaosEnsues-body shape this ticket lowered must be gone.
func TestPlanechasePlaneBodiesRefused(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range planechasePlaneBodies {
		names = append(names, f.card)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, f := range planechasePlaneBodies {
		c := byName[f.card]
		if c == nil {
			t.Errorf("%s: not lowered", f.card)
			continue
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if f.want != "" && !strings.Contains(got, f.want) {
			t.Errorf("%s (%s): want refusal %q, got %q", f.card, f.form, f.want, got)
		}
		if f.gone != "" && strings.Contains(got, f.gone) {
			t.Errorf("%s (%s): refusal %q must be gone, got %q", f.card, f.form, f.gone, got)
		}
	}
}
