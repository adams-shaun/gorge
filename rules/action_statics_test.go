package rules

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Recollecting one inert static per candidate must fail this allocation
// budget. Both games have identical objects/options; only the static differs.
// No Caster/Activator filter is used: its player-spec parsing would add
// unrelated per-candidate allocations to the measurement.
func TestLegalActionsReusesActionStaticMembership(t *testing.T) {
	for _, tc := range []struct {
		name, line  string
		noAbilities bool
	}{
		{"cast", "S:Mode$ CantBeCast | ValidCard$ Creature\n", false},
		{"activation", "S:Mode$ CantBeActivated | ValidCard$ Creature\n", false},
		{"continuous", "S:Mode$ Continuous | Affected$ Creature\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := func(static string) *Engine {
				spell := card(t, "Name:Action probe\nManaCost:0\nTypes:Sorcery\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
				source := card(t, "Name:Action source\nManaCost:0\nTypes:Artifact\n"+
					"A:AB$ Mana | Cost$ T | Produced$ C\n"+
					"A:AB$ Draw | Cost$ 0 | Defined$ You | NumCards$ 1\nOracle:x\n")
				hand := make([]*cards.Card, 0, 25)
				if tc.noAbilities {
					// Isolate granted-ability discovery. Payable abilities or
					// spells also scan Continuous for life-instead-of-mana
					// payment, which is deliberately outside this optimization.
					source = card(t, "Name:Action source\nTypes:Artifact\nOracle:x\n")
				} else {
					for range 12 {
						hand = append(hand, spell)
					}
				}
				handCount := len(hand)
				for range 12 {
					hand = append(hand, source)
				}
				hand = append(hand, card(t, "Name:Static holder\nTypes:Artifact\n"+static+"Oracle:x\n"))
				e := handEngine(t, hand...)
				ids := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)[handCount:]...)
				for _, id := range ids {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
				}
				return e
			}
			control, probe := fixture(""), fixture(tc.line)
			want, got := control.legalActions(0), probe.legalActions(0)
			wantEach := 12
			if tc.noAbilities {
				wantEach = 0
			}
			for _, kind := range []string{"cast", "activate", "ability"} {
				if n := kinds(want)[kind]; n != wantEach {
					t.Fatalf("control %s options = %d, want %d", kind, n, wantEach)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("inert static changed ordered options:\ngot  %+v\nwant %+v", got, want)
			}
			controlAllocs := testing.AllocsPerRun(100, func() {
				costStaticOptionsSink = control.legalActions(0)
			})
			probeAllocs := testing.AllocsPerRun(100, func() {
				costStaticOptionsSink = probe.legalActions(0)
			})
			extra := probeAllocs - controlAllocs
			t.Logf("control %.0f, static %.0f, extra %.0f allocations", controlAllocs, probeAllocs, extra)
			if extra > 3 {
				t.Fatalf("one action static added %.0f allocations per legal-actions pass; want at most 3 (one call-scoped collection)", extra)
			}
		})
	}
}

// A snapshot retained beyond legalActions would miss a departed/restored
// source; activation/payment callers must independently see the current set.
func TestActionStaticMembershipRefreshesAfterEvents(t *testing.T) {
	for _, tc := range []struct {
		name, target, static, kind string
		present, absent            int
	}{
		{"cast", "Types:Sorcery\nA:SP$ Draw | Defined$ You | NumCards$ 1\n",
			"S:Mode$ CantBeCast | ValidCard$ Sorcery\n", "cast", 0, 1},
		{"activation", "Types:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C\n",
			"S:Mode$ CantBeActivated | ValidCard$ Artifact\n", "activate", 0, 1},
		{"grant", "Types:Artifact\n",
			"S:Mode$ Continuous | Affected$ Artifact.Other | AddAbility$ Grant\n" +
				"SVar:Grant:AB$ Mana | Cost$ T | Produced$ C\n", "activate", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := card(t, "Name:Refresh target\nManaCost:0\n"+tc.target+"Oracle:x\n")
			holder := card(t, "Name:Refresh holder\nTypes:Artifact\n"+tc.static+"Oracle:x\n")
			e := handEngine(t, target, holder)
			ids := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
			if tc.kind != "cast" {
				e.emit(events.Event{Kind: events.MoveZone, Obj: ids[0], From: state.ZHand, To: state.ZBattlefield})
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: ids[1], From: state.ZHand, To: state.ZBattlefield})
			check := func(want int) {
				t.Helper()
				n := 0
				for _, opt := range e.legalActions(0) {
					if opt.Kind == tc.kind && opt.Obj == ids[0] {
						n++
					}
				}
				if n != want {
					t.Fatalf("%s target options = %d, want %d", tc.kind, n, want)
				}
				if tc.kind == "cast" {
					if restricted := e.castRestricted(0, ids[0]); restricted != (want == 0) {
						t.Fatalf("direct cast restriction = %v, want %v", restricted, want == 0)
					}
				} else if n := len(e.availableManaAbilities(0, ids[0])); n != want {
					t.Fatalf("direct mana membership = %d, want %d", n, want)
				}
			}
			check(tc.present)
			e.emit(events.Event{Kind: events.MoveZone, Obj: ids[1], From: state.ZBattlefield, To: state.ZGraveyard})
			check(tc.absent)
			e.emit(events.Event{Kind: events.MoveZone, Obj: ids[1], From: state.ZGraveyard, To: state.ZBattlefield})
			check(tc.present)
		})
	}
}

// Reusing evaluated matches (rather than just membership) would confuse the
// two candidates' mana values or retain a stale chosen-number gate.
func TestActionStaticMembershipKeepsDynamicMatches(t *testing.T) {
	zero := card(t, "Name:Zero\nManaCost:0\nTypes:Sorcery\nOracle:x\n")
	one := card(t, "Name:One\nManaCost:1\nTypes:Sorcery\nOracle:x\n")
	holder := card(t, "Name:Number gate\nTypes:Artifact\nS:Mode$ CantBeCast | ValidCard$ Sorcery.cmcEQChosen\nOracle:x\n")
	e := handEngine(t, zero, one, holder)
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	e.emit(events.Event{Kind: events.MoveZone, Obj: ids[2], From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "C")
	for _, n := range []int32{0, 1, 0} {
		e.emit(events.Event{Kind: events.Choose, Obj: ids[2], Counter: "number", Amount: n})
		opts := e.legalActions(0)
		if hasCastOption(opts, ids[0]) != (n != 0) || hasCastOption(opts, ids[1]) != (n != 1) {
			t.Fatalf("chosen number %d did not independently filter mana values: %+v", n, opts)
		}
	}
}

// A different seat/static traversal changes mana-choice indices. Expanding
// an unlocked Room's alternate face here would also change legacy legality:
// activeStatics (unlike staticEffects) reads only the current active face.
func TestActionStaticMembershipPreservesOrderAndActiveFace(t *testing.T) {
	target := card(t, "Name:Grant recipient\nTypes:Artifact\nOracle:x\n")
	spell := card(t, "Name:Room probe\nManaCost:0\nTypes:Sorcery\nOracle:x\n")
	room := card(t, "Name:Front grants\nTypes:Enchantment Room\n"+
		"S:Mode$ Continuous | Affected$ Artifact | AddAbility$ First\n"+
		"S:Mode$ Continuous | Affected$ Artifact | AddAbility$ Second\n"+
		"SVar:First:AB$ Mana | Cost$ T | Produced$ W\n"+
		"SVar:Second:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n"+
		"ALTERNATE\nName:Back grant\nTypes:Enchantment Room\n"+
		"S:Mode$ CantBeCast | ValidCard$ Sorcery\n"+
		"S:Mode$ Continuous | Affected$ Artifact | AddAbility$ Back\n"+
		"SVar:Back:AB$ Mana | Cost$ T | Produced$ B\nOracle:x\nAlternateMode:Split\n")
	other := card(t, "Name:Other seat grant\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ Artifact | AddAbility$ Other\n"+
		"SVar:Other:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	e := handEngine(t, target, spell, room, other)
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	// Insert the second seat's grant first: seat order, not object creation
	// or insertion order across seats, must put its R ability last.
	for _, id := range []state.ObjID{ids[3], ids[0], ids[2]} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	}
	e.emit(events.Event{Kind: events.ControlChange, Obj: ids[3], Player: 1})
	e.emit(events.Event{Kind: events.DoorUnlock, Obj: ids[2]})
	for face, want := range [][]string{{"W", "U", "R"}, {"B", "R"}} {
		e.emit(events.Event{Kind: events.FlipFace, Obj: ids[2], Amount: int32(face)})
		statics := actionStaticSource{e: e}
		for _, abilities := range [][]*cards.SA{
			e.availableManaAbilitiesUsing(&statics, 0, ids[0]),
			e.availableManaAbilities(0, ids[0]),
		} {
			var got []string
			for _, ability := range abilities {
				got = append(got, ability.Params["Produced"])
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("face %d: mana order = %v, want %v", face, got, want)
			}
		}
		if castable := hasCastOption(e.legalActions(0), ids[1]); castable != (face == 0) {
			t.Fatalf("face %d: room probe castable = %v, want %v", face, castable, face == 0)
		}
	}
}
