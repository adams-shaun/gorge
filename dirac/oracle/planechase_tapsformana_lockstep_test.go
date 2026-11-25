package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A plane whose printed TapsForMana static functions from the command zone
// (Eloren Wilds' spelling, the plain ManaReflected body the kernel lowers): a
// mana tap on the battlefield adds one mana of the type it produced. The
// kernel prices the extra as a command-zone StatTapExtra static on the
// seat's face-up plane; gorge queues the plane's trigger as a CR 605.3b
// immediate triggered mana ability at the tap. The probes' mana costs make
// both sides pay by tapping lands, where the extra joins the production
// (CR 605.4a).
const tapsForManaLockPlaneSrc = `Name:Tapsfor Mana Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ TapsForMana | ValidCard$ Permanent | Execute$ TapReflect | TriggerZones$ Command | Static$ True | TriggerDescription$ Whenever a player taps a permanent for mana, they add one extra mana of a type it made.
SVar:TapReflect:DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$ Produced | Defined$ TriggeredActivator | SpellDescription$ an extra mana of a produced type
Oracle:Whenever a player taps a permanent for mana, they add one extra mana of a type it made.
`

var tapsPlaneDecks = [2][]cardset.Entry{
	{{Name: "Planechase Probe", Count: 12}, {Name: "Island", Count: 28}},
	{{Name: "Planechase Probe", Count: 12}, {Name: "Island", Count: 28}},
}

// tapsPlaneRun plays the probe decks with the TapsForMana plane as each
// seat's one-card planar deck, returning the number of divergent seeds
// (stopping at the first unless report). mutate edits the matchup table
// (the mutation test's hook).
func tapsPlaneRun(t *testing.T, seeds uint64, report bool, mutate func(*kop.Table)) int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	plane, diags := cards.ParseBytes("planechase_taps_plane.txt", []byte(tapsForManaLockPlaneSrc))
	if plane == nil {
		t.Fatalf("plane parse: %v", diags)
	}
	if d := plane.Link(); len(d) != 0 {
		t.Fatalf("plane link: %v", d)
	}
	probe, diags := cards.ParseBytes("planechase_probe.txt", []byte(planechaseProbeSrc))
	if probe == nil {
		t.Fatalf("probe parse: %v", diags)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("probe link: %v", d)
	}
	island, ok := reg.Lookup("Island")
	if !ok {
		t.Fatal("Island not in the corpus")
	}
	pd, err := kgen.TableFromCards(reg, []*cards.Card{plane, probe, island}, "planechase-taps-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	// A zero-count entry puts the plane in the matchup table without dealing
	// it: the planar deck is genesis configuration, not a deck card.
	addPlane := func(es []cardset.Entry) []cardset.Entry {
		return append(es, cardset.Entry{Name: "Tapsfor Mana Probe Plane"})
	}
	tbl, _, _, err := set.Matchup(addPlane(tapsPlaneDecks[0]), addPlane(tapsPlaneDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	if !report {
		planeDef := tapsPlaneDef(t, tbl)
		if !tapsPlaneCommandExtra(planeDef) {
			t.Fatalf("the plane's table entry carries no command-zone reflected TapExtra static: types=%#x abilities=%+v", planeDef.Types, planeDef.Abilities)
		}
		if planeDef.Types&kop.TVanguard != 0 {
			t.Fatal("the plane face was marked TVanguard: its plane type must stay")
		}
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range tapsPlaneDecks[s] {
			c := island
			if e.Name == "Planechase Probe" {
				c = probe
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	played := map[string]int{}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchupPlanar(reg, [2]string{"planechase-taps-a", "planechase-taps-b"}, gd, [2][]*cards.Card{}, [2][]*cards.Card{{plane}, {plane}}, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, played), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			div++
			if !report {
				from := max(res.Div.At-12, 0)
				t.Errorf("seed %d last actions %v\nseed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					seed, actionStrings(res.Actions[from:res.Div.At]), seed, res.Div.At, res.Div.What,
					res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
				return div
			}
		}
	}
	if !report && played["Planechase Probe"] == 0 {
		t.Fatal("the probe was never cast: no mana tap ever met the plane's extra")
	}
	return div
}

// tapsPlaneDef is the matchup table's probe plane record.
func tapsPlaneDef(t *testing.T, tbl *kop.Table) *kop.CardDef {
	t.Helper()
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Tapsfor Mana Probe Plane" {
			return &tbl.Cards[i]
		}
	}
	t.Fatal("the probe plane is not in the matchup's table")
	return nil
}

// tapsPlaneCommandExtra reports whether def carries the plane's command-zone
// reflected tap extra.
func tapsPlaneCommandExtra(def *kop.CardDef) bool {
	for _, a := range def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra &&
			a.Static.Zone == kop.ZoneCommand && a.Static.StFlags&kop.StTapReflect != 0 &&
			a.Static.StFlags&kop.StTapAnyPlayer != 0 {
			return true
		}
	}
	return false
}

// TestPlanechaseTapsForManaLockstep: every mana tap adds the face-up plane's
// reflected extra on both engines, decision-for-decision.
func TestPlanechaseTapsForManaLockstep(t *testing.T) {
	t.Parallel()
	if n := tapsPlaneRun(t, 4, false, nil); n != 0 {
		t.Fatalf("%d divergent seeds", n)
	}
}

// TestPlanechaseTapsForManaMutation drops the command-zone StatTapExtra from
// the kernel's table and requires a divergence, so the seeds exercise the
// static rather than passing because no mana tap ever met the plane.
func TestPlanechaseTapsForManaMutation(t *testing.T) {
	t.Parallel()
	dropped := 0
	n := tapsPlaneRun(t, 8, true, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			var keep []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra && a.Static.Zone == kop.ZoneCommand {
					dropped++
					continue
				}
				keep = append(keep, a)
			}
			cards[i].Abilities = keep
		}
		tbl.Cards = cards
	})
	if dropped == 0 {
		t.Fatal("the kernel's table carries no command-zone StatTapExtra static to drop")
	}
	if n == 0 {
		t.Fatal("no seed diverged with the plane's command-zone tap-extra static dropped: the pool does not exercise the card")
	}
}
