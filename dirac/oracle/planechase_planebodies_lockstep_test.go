package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The planebodies ticket's ChaosEnsues-body shapes, played in lockstep on
// synthetic probe planes. The corpus planes themselves stay refused on their
// TapsForMana shapes (the tapsformana ticket's: dirac/kgen
// TestPlanechasePlaneBodiesRefused), so the bodies lower under the planes'
// own printed SVar lines with no TapsForMana line beside them:
//
//   - the dig plane is The Fertile Lands of Saulvinia's chaos body verbatim:
//     its dig reveals the planar deck's face-down remainder (the face-up
//     current plane at the head is not part of the reveal), remembers the
//     found plane, moves it to the deck's bottom, and the chained DB$
//     ChaosEnsues | Defined$ Remembered erupts that plane -- bottom seat,
//     face down (gorge planar.go's membership boundary) -- before the
//     Cleanup clears the remember;
//   - the draw plane is a plane whose chaos ability draws a card, so the
//     remembered-plane eruption and every deck rotation are observable in
//     the games' hands.
//
// Both engines must queue the same chaos, rotate the planar deck the same
// way and draw the same cards.
const planebodiesDigPlaneSrc = `Name:Planebodies Dig Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ PlanebodiesDig | TriggerDescription$ chaos: dig the planar deck, erupt the found plane
SVar:PlanebodiesDig:DB$ DigUntil | Valid$ Plane | DigZone$ PlanarDeck | RememberFound$ True | FoundDestination$ PlanarDeck | FoundLibraryPosition$ -1 | RevealedDestination$ PlanarDeck | RevealedLibraryPosition$ -1 | SubAbility$ PlanebodiesChaos
SVar:PlanebodiesChaos:DB$ ChaosEnsues | Defined$ Remembered | SubAbility$ PlanebodiesCleanup
SVar:PlanebodiesCleanup:DB$ Cleanup | ClearRemembered$ True
Oracle:chaos: dig the planar deck, erupt the found plane
`

const planebodiesDrawPlaneSrc = `Name:Planebodies Draw Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ ChaosProbeDraw | TriggerDescription$ chaos: the probe's player draws
SVar:ChaosProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:chaos: the probe's player draws
`

const planebodiesProbeSrc = `Name:Planebodies Chaos Probe
ManaCost:1
Types:Sorcery
A:SP$ ChaosEnsues | Defined$ You | SpellDescription$ chaos erupts
Oracle:chaos erupts
`

var planebodiesDecks = [2][]cardset.Entry{
	{{Name: "Planebodies Chaos Probe", Count: 12}, {Name: "Island", Count: 28}},
	{{Name: "Planebodies Chaos Probe", Count: 12}, {Name: "Island", Count: 28}},
}

// TestPlanechasePlaneBodiesLockstep plays the chaos probe beside a
// three-plane planar deck on each seat: the dig plane's planar dig erupts a
// draw plane's chaos from its bottom seat and rotates the deck, and the game
// stays decision-for-decision with gorge.
func TestPlanechasePlaneBodiesLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	dig, diags := cards.ParseBytes("planebodies_dig_plane.txt", []byte(planebodiesDigPlaneSrc))
	if dig == nil {
		t.Fatalf("dig plane parse: %v", diags)
	}
	draw, diags := cards.ParseBytes("planebodies_draw_plane.txt", []byte(planebodiesDrawPlaneSrc))
	if draw == nil {
		t.Fatalf("draw plane parse: %v", diags)
	}
	probe, diags := cards.ParseBytes("planebodies_probe.txt", []byte(planebodiesProbeSrc))
	if probe == nil {
		t.Fatalf("probe parse: %v", diags)
	}
	if d := dig.Link(); len(d) != 0 {
		t.Fatalf("dig plane link: %v", d)
	}
	if d := draw.Link(); len(d) != 0 {
		t.Fatalf("draw plane link: %v", d)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("probe link: %v", d)
	}
	island, ok := reg.Lookup("Island")
	if !ok {
		t.Fatal("Island not in the corpus")
	}
	pd, err := kgen.TableFromCards(reg, []*cards.Card{dig, draw, probe, island}, "planebodies-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	// A zero-count entry puts the planes in the matchup table without dealing
	// them: the planar deck is genesis configuration, not a deck card.
	addPlane := func(es []cardset.Entry) []cardset.Entry {
		return append(es, cardset.Entry{Name: "Planebodies Dig Probe Plane"},
			cardset.Entry{Name: "Planebodies Draw Probe Plane"})
	}
	tbl, _, _, err := set.Matchup(addPlane(planebodiesDecks[0]), addPlane(planebodiesDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range planebodiesDecks[s] {
			c := probe
			if e.Name != "Planebodies Chaos Probe" {
				c = island
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	played := map[string]int{}
	// digs counts the dig plane's planar digs: each one moves the found plane
	// and the revealed rest within the planar deck (From=To=ZPlanarDeck
	// MoveZone events; the dig's arm is their only emitter), so a game where
	// no seed ever planeswalks to the dig plane would fail this count instead
	// of passing vacuously on a shuffle that happens to hide the dig.
	digs := 0
	for seed := uint64(1); seed <= 4; seed++ {
		m, err := NewMatchupPlanar(reg, [2]string{"planebodies-a", "planebodies-b"}, gd, [2][]*cards.Card{},
			[2][]*cards.Card{{dig, draw, draw}, {dig, draw, draw}}, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, played), RunConfig{
			Config: cfg, MaxDecisions: 3000, MaxTurns: 30,
			Finish: func(g *GorgeSide) {
				for i := range g.E.L.Events {
					e := &g.E.L.Events[i]
					if e.Kind == events.MoveZone && e.From == state.ZPlanarDeck && e.To == state.ZPlanarDeck {
						digs++
					}
				}
			},
		})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if played["Planebodies Chaos Probe"] == 0 {
		t.Fatal("the chaos probe was never cast")
	}
	if digs == 0 {
		t.Fatal("the dig plane's planar dig never ran in any seed (no planar-deck-to-planar-deck move)")
	}
}
