package rules

// ChangeType$ EACH: the per-type structured pick on the library-search,
// hand and hidden-pick paths, driven through a real engine.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	forestCardSrc = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
	plainsCardSrc = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"
)

// clearHandExcept returns every card in seat p's hand except keep to the
// library with logged MoveZones, so a hand-origin pick sees only the cards
// a test puts there.
func clearHandExcept(t *testing.T, e *Engine, p state.PlayerID, keep state.ObjID) {
	t.Helper()
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, p)...) {
		if id != keep {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary, Player: p})
		}
	}
}

// answerYes answers a yes/no confirmation with its "yes" option.
func answerYes(t *testing.T, e *Engine, resume string) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.ResumeKind != resume {
		t.Fatalf("pending = %+v, want the %s confirmation", d, resume)
	}
	for _, o := range d.Options {
		if o.Kind == "yes" {
			return answerKindFor(t, e, d.Kind, d.Player, o.Index)
		}
	}
	t.Fatalf("no yes option on %+v", d)
	return nil
}

const vergeBody = "A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ EACH Forest & Plains | Tapped$ True"

// TestKernelKrosanVergeEachSearchAsksPerTypeAndHonoursTheAnswer: Krosan Verge's
// search over a library holding Forests AND Plains asks the controller for
// 0..2 cards, the Forests grouped "0" and the Plainses "1" in library
// order; one-per-group is the only legal shape, and the answered Forest and
// Plains enter tapped before exactly one shuffle.
func TestKernelKrosanVergeEachSearchAsksPerTypeAndHonoursTheAnswer(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureGame(t, 150, sorceryFixtureSrc("VergeSearch", vergeBody),
		[]string{forestCardSrc, forestCardSrc, plainsCardSrc, plainsCardSrc}, nil)
	addMana(t, e, 0, "B")
	ids := stackLibraryTop(t, e, 0, "Forest", "Forest", "Plains", "Plains")
	d := castSpellObj(t, e, id)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" || d.Player != 0 {
		t.Fatalf("decision = %+v, want a KChoose search for seat 0", d)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("range = %d..%d, want 0..2", d.Min, d.Max)
	}
	if d.Prompt != "Search a library: choose one card of each listed type" {
		t.Fatalf("prompt = %q", d.Prompt)
	}
	if len(d.Options) != 4 {
		t.Fatalf("%d options, want 4: %+v", len(d.Options), d.Options)
	}
	for i, w := range []struct {
		id           state.ObjID
		group, label string
	}{{ids[0], "0", "Forest"}, {ids[1], "0", "Forest"}, {ids[2], "1", "Plains"}, {ids[3], "1", "Plains"}} {
		o := d.Options[i]
		if o.Obj != w.id || o.Group != w.group || o.Label != w.label || o.Kind != "search" {
			t.Fatalf("option %d = %+v, want obj %d group %q label %q", i, o, w.id, w.group, w.label)
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 1}}); err == nil {
		t.Fatal("two same-group choices accepted")
	}
	mark := len(e.L.Events)
	if p := answerKindFor(t, e, decision.KChoose, 0, 1, 2); p != nil {
		t.Fatalf("unexpected follow-up ask %+v", p)
	}
	for _, f := range []state.ObjID{ids[1], ids[2]} {
		if o := e.G.Obj(f); o.Zone != state.ZBattlefield || !o.Tapped {
			t.Fatalf("fetched land %d zone %s tapped %v, want a tapped permanent", f, o.Zone, o.Tapped)
		}
	}
	shuffles := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if shuffles != 1 {
		t.Fatalf("shuffles = %d, want 1", shuffles)
	}
	replayCheck(t, e, cfg)
}

// TestKernelEachSearchOffersOnlyTheTypesWithCandidates: with Forests and no
// Plains only the Forest group is offered (Max 1) and the answered Forest
// moves.
func TestKernelEachSearchOffersOnlyTheTypesWithCandidates(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureGame(t, 151, sorceryFixtureSrc("VergeSearch", vergeBody), []string{forestCardSrc, forestCardSrc}, nil)
	addMana(t, e, 0, "B")
	ids := stackLibraryTop(t, e, 0, "Forest", "Forest")
	d := castSpellObj(t, e, id)
	if d == nil || d.Min != 0 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("decision = %+v, want 0..1 over the Forest group only", d)
	}
	if d.Options[0].Group != "0" || d.Options[1].Group != "0" {
		t.Fatalf("groups = %q/%q, want one Forest group", d.Options[0].Group, d.Options[1].Group)
	}
	answerKindFor(t, e, decision.KChoose, 0, optionIdxOfObj(t, d, ids[1]))
	if zoneOfID(e, ids[1]) != state.ZBattlefield || zoneOfID(e, ids[0]) != state.ZLibrary {
		t.Fatalf("zones = %s/%s, want only the answered Forest moved", zoneOfID(e, ids[0]), zoneOfID(e, ids[1]))
	}
	replayCheck(t, e, cfg)
}

// requireEachGroups checks d offers exactly crea in group "0" and land in "1"
// (and never other), returning their option indices.
func requireEachGroups(t *testing.T, d *decision.Decision, crea, land, other state.ObjID) (int, int) {
	t.Helper()
	if d.Min != 0 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("decision = %+v, want 0..2 over one option per listed type", d)
	}
	ci, li := -1, -1
	for _, o := range d.Options {
		switch o.Obj {
		case crea:
			ci = o.Index
			if o.Group != "0" {
				t.Fatalf("creature group = %q, want \"0\"", o.Group)
			}
		case land:
			li = o.Index
			if o.Group != "1" {
				t.Fatalf("land group = %q, want \"1\"", o.Group)
			}
		case other:
			t.Fatal("a card matching neither listed type was offered")
		}
	}
	if ci < 0 || li < 0 {
		t.Fatalf("options %+v do not cover both listed types", d.Options)
	}
	return ci, li
}

// TestKernelEachHandMoveAsksOneOfEachListedType (Michelangelo Improvisers' shape):
// Optional$ True poses the confirm gate first, then a one-of-EACH pick
// (Max 2, one Group per type, the instant excluded), and the answered pair
// both enter the battlefield.
func TestKernelEachHandMoveAsksOneOfEachListedType(t *testing.T) {
	t.Parallel()
	body := "A:SP$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ EACH Creature & Land | Optional$ True"
	e, cfg, id := newFixtureGame(t, 152, sorceryFixtureSrc("Improvise", body), []string{
		"Name:Grizzly\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", forestCardSrc,
		"Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n"}, nil)
	addMana(t, e, 0, "B")
	clearHandExcept(t, e, 0, id)
	crea := putNamed(t, e, 0, "Grizzly", state.ZHand)
	land := putNamed(t, e, 0, "Forest", state.ZHand)
	inst := putNamed(t, e, 0, "Bolt", state.ZHand)
	castSpellObj(t, e, id)
	d := answerYes(t, e, "hand_move_confirm")
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "hand_move" {
		t.Fatalf("decision = %+v, want a KChoose hand_move", d)
	}
	ci, li := requireEachGroups(t, d, crea, land, inst)
	answerKindFor(t, e, decision.KChoose, 0, ci, li)
	if zoneOfID(e, crea) != state.ZBattlefield || zoneOfID(e, land) != state.ZBattlefield || zoneOfID(e, inst) != state.ZHand {
		t.Fatalf("zones crea/land/inst = %s/%s/%s, want battlefield/battlefield/hand", zoneOfID(e, crea), zoneOfID(e, land), zoneOfID(e, inst))
	}
	replayCheck(t, e, cfg)
}

// TestKernelEachHiddenPickAsksOneOfEachListedType (Druidic Ritual's shape): from
// the graveyard, the confirm gate then a one-of-EACH pick, and the answered
// creature and land both return to hand.
func TestKernelEachHiddenPickAsksOneOfEachListedType(t *testing.T) {
	t.Parallel()
	body := "A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Hand | Hidden$ True | ChangeType$ EACH Creature.YouOwn & Land.YouOwn | Optional$ True"
	e, cfg, id := newFixtureGame(t, 153, sorceryFixtureSrc("Ritual", body), []string{
		"Name:Zed\nManaCost:B\nTypes:Creature Zombie\nPT:2/2\nOracle:x\n", forestCardSrc,
		"Name:Page\nManaCost:U\nTypes:Instant\nOracle:x\n"}, nil)
	addMana(t, e, 0, "B")
	crea := putNamed(t, e, 0, "Zed", state.ZGraveyard)
	land := putNamed(t, e, 0, "Forest", state.ZGraveyard)
	other := putNamed(t, e, 0, "Page", state.ZGraveyard)
	castSpellObj(t, e, id)
	d := answerYes(t, e, "hidden_pick_confirm")
	if d == nil || d.ResumeKind != "hidden_pick" {
		t.Fatalf("decision = %+v, want a hidden_pick", d)
	}
	ci, li := requireEachGroups(t, d, crea, land, other)
	answerKindFor(t, e, d.Kind, d.Player, ci, li)
	if zoneOfID(e, crea) != state.ZHand || zoneOfID(e, land) != state.ZHand || zoneOfID(e, other) != state.ZGraveyard {
		t.Fatalf("zones = %s/%s/%s, want hand/hand/graveyard", zoneOfID(e, crea), zoneOfID(e, land), zoneOfID(e, other))
	}
	replayCheck(t, e, cfg)
}

// TestKernelEachExplicitZeroChangeNumSelectsNothing: an explicit ChangeNum$ 0 on
// the EACH hand, library-search and hidden-pick paths poses no pick (the
// Optional$ confirm gate still runs first where present) and moves nothing.
func TestKernelEachExplicitZeroChangeNumSelectsNothing(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct {
		name, body, confirm string
		zone                state.Zone
	}{
		{"hand", "Origin$ Hand | Destination$ Battlefield | Optional$ True", "hand_move_confirm", state.ZHand},
		{"library_search", "Origin$ Library | Destination$ Hand", "", state.ZLibrary},
		{"hidden_pick", "Origin$ Graveyard | Destination$ Hand | Hidden$ True | Optional$ True", "hidden_pick_confirm", state.ZGraveyard},
	} {
		tc := tc
		seed := uint64(154 + i)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := "A:SP$ ChangeZone | " + tc.body + " | ChangeType$ EACH Creature & Land.Forest | ChangeNum$ 0"
			e, cfg, id := newFixtureGame(t, seed, sorceryFixtureSrc("ZeroProbe", body), []string{
				"Name:ZeroCreature\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", forestCardSrc}, nil)
			addMana(t, e, 0, "B")
			clearHandExcept(t, e, 0, id)
			var crea, land state.ObjID
			if tc.zone == state.ZLibrary {
				ids := stackLibraryTop(t, e, 0, "ZeroCreature", "Forest")
				crea, land = ids[0], ids[1]
			} else {
				crea = putNamed(t, e, 0, "ZeroCreature", tc.zone)
				land = putNamed(t, e, 0, "Forest", tc.zone)
			}
			d := castSpellObj(t, e, id)
			if tc.confirm != "" {
				d = answerYes(t, e, tc.confirm)
			}
			if d != nil {
				t.Fatalf("zero-count EACH posed a choice: %+v", d)
			}
			for _, c := range []state.ObjID{crea, land} {
				if z := zoneOfID(e, c); z != tc.zone {
					t.Fatalf("candidate %d ended in %s, want it to remain in %s", c, z, tc.zone)
				}
			}
			if hasNote(e, "unimplemented API ChangeZone") {
				t.Fatal("the ChangeZone handler did not run")
			}
			replayCheck(t, e, cfg)
		})
	}
}
