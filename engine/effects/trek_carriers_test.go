package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Unicomplex is the corpus carrier of an absent LibraryPosition$ whose
// card is a land with an enters-untapped mode: "put target creature card from
// your graveyard on top of your library". The compiled mode body, resolved
// the way a chosen mode is, must place the creature on top.
func TestUnicomplexModePutsTheCreatureOnTop(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("The Unicomplex")
	if !ok {
		t.Fatal("corpus has no The Unicomplex")
	}
	mode := cards.ResolveSVar(card.Faces[0].SVars, "DBChangeZone")
	if mode == nil || mode.API != "ChangeZone" {
		t.Fatalf("The Unicomplex has no ChangeZone mode body: %+v", mode)
	}
	if mode.Params["LibraryPosition"] != "" || mode.Params["Origin"] != "Graveyard" || mode.Params["Destination"] != "Library" {
		t.Fatalf("precondition: mode params = %v, want Origin$ Graveyard, Destination$ Library, no LibraryPosition$", mode.Params)
	}
	h := newHost(t, 2)
	fills := lzFills(t, h, 2)
	source := lzAdd(t, h, 0, state.ZBattlefield, card)
	tgt := lzCreature(t, h, 0, state.ZGraveyard, "Vanished")
	Resolve(h, &Ctx{Controller: 0, Source: source, TargetsOffered: true,
		Targets: []state.Target{{Obj: tgt}}}, mode)
	if z := h.g.Obj(tgt).Zone; z != state.ZLibrary {
		t.Fatalf("creature zone = %v, want library", z)
	}
	want := append([]state.ObjID{tgt}, fills...)
	if lib := lzLibraryOrderIDs(h, 0); !slices.Equal(lib, want) {
		t.Fatalf("library = %v, want %v (on top)", lib, want)
	}
}

// Assimilation Assessor's enters trigger sweeps a target opponent's graveyard
// into exile (ChangeZoneAll | ValidTgts$ Opponent): the targeted player's
// graveyard empties, the other player's does not.
func TestAssimilationAssessorExilesTheTargetOpponentsGraveyard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Assimilation Assessor")
	if !ok {
		t.Fatal("corpus has no Assimilation Assessor")
	}
	body := cards.ResolveSVar(card.Faces[0].SVars, "TrigExile")
	if body == nil || body.API != "ChangeZoneAll" || body.Params["ValidTgts"] != "Opponent" {
		t.Fatalf("precondition: trigger body = %+v, want ChangeZoneAll with ValidTgts$ Opponent", body)
	}
	h := newHost(t, 2)
	source := lzAdd(t, h, 0, state.ZBattlefield, card)
	theirs := lzCreature(t, h, 1, state.ZGraveyard, "Their Dead")
	mine := lzCreature(t, h, 0, state.ZGraveyard, "My Dead")
	Resolve(h, &Ctx{Controller: 0, Source: source, TargetsOffered: true,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, body)
	if z := h.g.Obj(theirs).Zone; z != state.ZExile {
		t.Fatalf("the opponent's graveyard card is in %v, want exile", z)
	}
	if z := h.g.Obj(mine).Zone; z != state.ZGraveyard {
		t.Fatalf("the controller's own graveyard card is in %v, want graveyard untouched", z)
	}
}

// Chief Montgomery Scott's three ChangeZone sub-abilities each carry the
// condition on the chosen card's kind, and the comma entry list of the last
// one ("P1P1,CHARGE") places each kind as its own counter, X of each.
func TestChiefScottEntersWithTheCountersOfItsKind(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	scott, ok := reg.Lookup("Chief Montgomery Scott")
	if !ok {
		t.Fatal("corpus has no Chief Montgomery Scott")
	}
	face := scott.Faces[0]
	head := cards.ResolveSVar(face.SVars, "DBPutCreature")
	if head == nil {
		t.Fatal("Chief Montgomery Scott has no DBPutCreature body")
	}
	cases := []struct {
		name, src string
		p1p1, chg int32
	}{
		{"creature", "Name:Plain Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", 2, 0},
		{"spacecraft", "Name:Plain Ship\nTypes:Artifact Spacecraft\nOracle:x\n", 0, 2},
		{"creature spacecraft", "Name:Living Ship\nTypes:Artifact Creature Spacecraft\nPT:2/2\nOracle:x\n", 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			source := lzAdd(t, h, 0, state.ZBattlefield, scott)
			for _, land := range []string{"Island", "Forest", "Forest"} {
				lzAdd(t, h, 0, state.ZBattlefield, mkCard(t, "Name:"+land+"\nTypes:Basic Land "+land+"\nOracle:x\n"))
			}
			chosen := lzAdd(t, h, 0, state.ZHand, mkCard(t, tc.src))
			c := &Ctx{Controller: 0, Source: source, SVars: face.SVars,
				Chosen: []state.Target{{Obj: chosen}}}
			Resolve(h, c, head)
			o := h.g.Obj(chosen)
			if o.Zone != state.ZBattlefield {
				t.Fatalf("zone = %v, want battlefield", o.Zone)
			}
			if got := o.Counter("P1P1"); got != tc.p1p1 {
				t.Errorf("P1P1 counters = %d, want %d (counters %v)", got, tc.p1p1, o.Counters)
			}
			if got := o.Counter("CHARGE"); got != tc.chg {
				t.Errorf("CHARGE counters = %d, want %d (counters %v)", got, tc.chg, o.Counters)
			}
		})
	}
}

// Sins of the Father's third chapter exiles a target player's graveyard
// (ChangeZoneAll | ValidTgts$ Player) and each opponent then loses life equal
// to the creature cards exiled: two creature cards and a land in the targeted
// opponent's graveyard cost them 2, and the graveyard is empty afterwards.
func TestSinsOfTheFatherExilesTheTargetGraveyardAndCountsCreatures(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Sins of the Father")
	if !ok {
		t.Fatal("corpus has no Sins of the Father")
	}
	face := card.Faces[0]
	body := cards.ResolveSVar(face.SVars, "DBExile")
	if body == nil || body.API != "ChangeZoneAll" || body.Params["ValidTgts"] != "Player" {
		t.Fatalf("precondition: chapter body = %+v, want ChangeZoneAll with ValidTgts$ Player", body)
	}
	h := newHost(t, 2)
	source := lzAdd(t, h, 0, state.ZBattlefield, card)
	c1 := lzCreature(t, h, 1, state.ZGraveyard, "Dead One")
	c2 := lzCreature(t, h, 1, state.ZGraveyard, "Dead Two")
	land := lzAdd(t, h, 1, state.ZGraveyard, mkCard(t, "Name:Dead Land\nTypes:Land\nOracle:x\n"))
	before := h.g.Players[1].Life
	Resolve(h, &Ctx{Controller: 0, Source: source, SVars: face.SVars, TargetsOffered: true,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, body)
	for _, id := range []state.ObjID{c1, c2, land} {
		if z := h.g.Obj(id).Zone; z != state.ZExile {
			t.Fatalf("object %d is in %v, want exile", id, z)
		}
	}
	if got := before - h.g.Players[1].Life; got != 2 {
		t.Fatalf("the opponent lost %d life, want 2 (the creature cards exiled)", got)
	}
}
