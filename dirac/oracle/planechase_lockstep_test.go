package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A plane in the kernel's object space: the planar deck's face-up current
// plane carries a Mode$ ChaosEnsues trigger in the command zone, and the
// synthetic probe's DB$ ChaosEnsues erupts it. Both engines must queue the
// same trigger and draw the same card.
const planechasePlaneSrc = `Name:Planechase Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ ChaosProbeDraw | TriggerDescription$ Whenever chaos ensues, draw a card.
SVar:ChaosProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:Whenever chaos ensues, draw a card.
`

const planechaseProbeSrc = `Name:Planechase Probe
ManaCost:1
Types:Sorcery
A:SP$ ChaosEnsues | Defined$ You | SpellDescription$ Chaos ensues.
Oracle:Chaos ensues.
`

var planechaseDecks = [2][]cardset.Entry{
	{{Name: "Planechase Probe", Count: 12}, {Name: "Island", Count: 28}},
	{{Name: "Planechase Probe", Count: 12}, {Name: "Island", Count: 28}},
}

// TestPlanechaseLockstep plays the synthetic probe beside a one-card planar
// deck on each seat: the probe erupts the face-up plane's chaos ability, and
// the game stays decision-for-decision with gorge.
func TestPlanechaseLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	plane, diags := cards.ParseBytes("planechase_plane.txt", []byte(planechasePlaneSrc))
	if plane == nil {
		t.Fatalf("plane parse: %v", diags)
	}
	probe, diags := cards.ParseBytes("planechase_probe.txt", []byte(planechaseProbeSrc))
	if probe == nil {
		t.Fatalf("probe parse: %v", diags)
	}
	if d := plane.Link(); len(d) != 0 {
		t.Fatalf("plane link: %v", d)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("probe link: %v", d)
	}
	island, ok := reg.Lookup("Island")
	if !ok {
		t.Fatal("Island not in the corpus")
	}
	pd, err := kgen.TableFromCards(reg, []*cards.Card{plane, probe, island}, "planechase-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	// A zero-count entry puts the plane in the matchup table without dealing
	// it: the planar deck is genesis configuration, not a deck card.
	addPlane := func(es []cardset.Entry) []cardset.Entry {
		return append(es, cardset.Entry{Name: "Planechase Probe Plane"})
	}
	tbl, _, _, err := set.Matchup(addPlane(planechaseDecks[0]), addPlane(planechaseDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range planechaseDecks[s] {
			c := probe
			if e.Name != "Planechase Probe" {
				c = island
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	played := map[string]int{}
	for seed := uint64(1); seed <= 4; seed++ {
		m, err := NewMatchupPlanar(reg, [2]string{"planechase-a", "planechase-b"}, gd, [2][]*cards.Card{}, [2][]*cards.Card{{plane}, {plane}}, 1)
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
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if played["Planechase Probe"] == 0 {
		t.Fatal("the chaos probe was never cast")
	}
}
