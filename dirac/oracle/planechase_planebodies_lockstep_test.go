package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The planebodies ticket's ChaosEnsues-body shapes, played in lockstep on
// synthetic probe planes. The corpus planes themselves stay refused on their
// TapsForMana shapes (the tapsformana ticket's: dirac/kgen
// TestPlanechasePlaneBodiesRefused), so the bodies lower under the planes'
// own printed SVar lines with no TapsForMana line beside them:
//
//   - the dig plane is The Fertile Lands of Saulvinia's chaos body verbatim:
//     its dig reveals the planar deck's non-current remainder (the current
//     plane at the head is not part of the reveal), remembers the
//     found plane, moves it to the deck's bottom, and the chained DB$
//     ChaosEnsues | Defined$ Remembered erupts that plane -- bottom seat,
//     face up (gorge planar.go's membership boundary) -- before the
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

// planebodiesCursePlaneSrc is Eloren Wilds' chaos body verbatim in structure
// (its T[0] reflected command-zone TapsForMana trigger stays the tapsformana
// ticket's refusal): chaos ensues, target player gets the DB$ Effect whose
// STCantCast static (CantBeCast | ValidCard$ Card | Caster$
// Player.IsRemembered) stops that player's casts and whose unscoped
// PlaneswalkedTo self-exile trigger (the ChangeZone Self Command->Exile idiom)
// ends it at the first planeswalk. The effect lives over the plane as its
// source; the curse is the plane-source Effect record's whole behaviour.
const planebodiesCursePlaneSrc = `Name:Planebodies Curse Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ RolledChaos | TriggerDescription$ chaos: target player can't cast spells until a player planeswalks.
SVar:RolledChaos:DB$ Effect | StaticAbilities$ STCantCast | Triggers$ TrigPlaneswalk | RememberObjects$ Targeted | ValidTgts$ Player | IsCurse$ True | Duration$ Permanent
SVar:STCantCast:Mode$ CantBeCast | Caster$ Player.IsRemembered | ValidCard$ Card | Description$ Target player can't cast spells until a player planeswalks.
SVar:TrigPlaneswalk:Static$ True | Mode$ PlaneswalkedTo | Execute$ ExileSelf
SVar:ExileSelf:DB$ ChangeZone | Destination$ Exile | Origin$ Command | Defined$ Self
Oracle:chaos: target player can't cast spells until a player planeswalks.
`

const planebodiesProbeSrc = `Name:Planebodies Chaos Probe
ManaCost:1
Types:Sorcery
A:SP$ ChaosEnsues | Defined$ You | SpellDescription$ chaos erupts
Oracle:chaos erupts
`

const planebodiesWalkProbeSrc = `Name:Planebodies Walk Probe
ManaCost:1
Types:Sorcery
A:SP$ Planeswalk | Defined$ You | SpellDescription$ planeswalk
Oracle:planeswalk
`

var planebodiesDecks = [2][]cardset.Entry{
	{{Name: "Planebodies Chaos Probe", Count: 12}, {Name: "Planebodies Walk Probe", Count: 6}, {Name: "Island", Count: 22}},
	{{Name: "Planebodies Chaos Probe", Count: 12}, {Name: "Planebodies Walk Probe", Count: 6}, {Name: "Island", Count: 22}},
}

// TestPlanechasePlaneBodiesLockstep plays the chaos probe beside a
// four-plane planar deck on each seat: the dig plane's planar dig erupts a
// draw plane's chaos from its bottom seat and rotates the deck, the curse
// plane's chaos grants Eloren's plane-source Effect over a player (the cast
// gate stops that player's probes until a planeswalk ends it, the walk
// probe's cast), and the game stays decision-for-decision with gorge.
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
	curse, diags := cards.ParseBytes("planebodies_curse_plane.txt", []byte(planebodiesCursePlaneSrc))
	if curse == nil {
		t.Fatalf("curse plane parse: %v", diags)
	}
	probe, diags := cards.ParseBytes("planebodies_probe.txt", []byte(planebodiesProbeSrc))
	if probe == nil {
		t.Fatalf("probe parse: %v", diags)
	}
	walk, diags := cards.ParseBytes("planebodies_walk_probe.txt", []byte(planebodiesWalkProbeSrc))
	if walk == nil {
		t.Fatalf("walk probe parse: %v", diags)
	}
	if d := dig.Link(); len(d) != 0 {
		t.Fatalf("dig plane link: %v", d)
	}
	if d := draw.Link(); len(d) != 0 {
		t.Fatalf("draw plane link: %v", d)
	}
	if d := curse.Link(); len(d) != 0 {
		t.Fatalf("curse plane link: %v", d)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("probe link: %v", d)
	}
	if d := walk.Link(); len(d) != 0 {
		t.Fatalf("walk probe link: %v", d)
	}
	island, ok := reg.Lookup("Island")
	if !ok {
		t.Fatal("Island not in the corpus")
	}
	pd, err := kgen.TableFromCards(reg, []*cards.Card{dig, draw, curse, probe, walk, island}, "planebodies-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	// A zero-count entry puts the planes in the matchup table without dealing
	// them: the planar deck is genesis configuration, not a deck card.
	addPlane := func(es []cardset.Entry) []cardset.Entry {
		return append(es, cardset.Entry{Name: "Planebodies Dig Probe Plane"},
			cardset.Entry{Name: "Planebodies Draw Probe Plane"},
			cardset.Entry{Name: "Planebodies Curse Probe Plane"})
	}
	tbl, _, _, err := set.Matchup(addPlane(planebodiesDecks[0]), addPlane(planebodiesDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range planebodiesDecks[s] {
			c := probe
			switch e.Name {
			case "Planebodies Walk Probe":
				c = walk
			case "Island":
				c = island
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	played := map[string]int{}
	inner := castActivatePlayedPolicy(1, played)
	// The curse poll reads the gorge side's cast gate every decision: a hand
	// probe the gate refuses means Eloren's Effect is live over that seat;
	// a later decision where the same seat's probe is offered again means a
	// planeswalk ended it.
	cursed := [2]bool{}
	curseSeen, curseReleased := false, false
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for p := 0; p < 2; p++ {
				for _, id := range c.G.E.Game().Zone(state.ZHand, state.PlayerID(p)) {
					o := c.G.E.Game().Obj(id)
					if o == nil || o.Face() == nil || o.Face().Name != "Planebodies Chaos Probe" {
						continue
					}
					if c.G.E.CastProhibited(state.PlayerID(p), id) {
						cursed[p] = true
						curseSeen = true
					} else if cursed[p] {
						curseReleased = true
					}
				}
			}
		}
		return inner.Pick(c)
	})
	for seed := uint64(1); seed <= 4; seed++ {
		m, err := NewMatchupPlanar(reg, [2]string{"planebodies-a", "planebodies-b"}, gd, [2][]*cards.Card{},
			[2][]*cards.Card{{dig, draw, draw, curse}, {dig, draw, draw, curse}}, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if played["Planebodies Chaos Probe"] == 0 {
		t.Fatal("the chaos probe was never cast")
	}
	if played["Planebodies Walk Probe"] == 0 {
		t.Fatal("the walk probe was never cast")
	}
	if !curseSeen {
		t.Fatal("the curse plane's chaos never granted the Effect")
	}
	if !curseReleased {
		t.Fatal("no planeswalk ever ended a live curse")
	}
}
